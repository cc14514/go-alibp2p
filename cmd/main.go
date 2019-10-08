package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/cc14514/go-alibp2p"
	"github.com/cc14514/go-lightrpc/rpcserver"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/urfave/cli"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"
)

const echopid = "/echo/1.0.0"

var (
	Stop                     = make(chan struct{})
	homedir, bootnodes       string
	port, networkid, rpcport int
	nodiscover               bool
	p2pservice               *alibp2p.Service
	ServiceRegMap            = make(map[string]rpcserver.ServiceReg)
	genServiceReg            = func(namespace, version string, service interface{}) {
		ServiceRegMap[namespace] = rpcserver.ServiceReg{
			Namespace: namespace,
			Version:   version,
			Service:   service,
		}
	}
)

func init() {
	vsn := "0.0.1"
	genServiceReg("shell", vsn, &shellservice{})
}

func main() {
	app := cli.NewApp()
	app.Name = os.Args[0]
	app.Usage = "JSON-RPC 接口框架"
	app.Version = "0.0.1"
	app.Author = "liangc"
	app.Email = "cc14514@icloud.com"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "nodiscover",
			Usage:       "false to disable bootstrap",
			Destination: &nodiscover,
		},
		cli.IntFlag{
			Name:        "rpcport",
			Usage:       "HTTP-RPC server listening `PORT`",
			Value:       8080,
			Destination: &rpcport,
		},
		cli.IntFlag{
			Name:        "port",
			Usage:       "service tcp port",
			Value:       10000,
			Destination: &port,
		},
		cli.IntFlag{
			Name:        "networkid",
			Usage:       "network id",
			Value:       1,
			Destination: &networkid,
		},
		cli.StringFlag{
			Name:        "homedir,d",
			Usage:       "home dir",
			Value:       "/tmp",
			Destination: &homedir,
		},
		cli.StringFlag{
			Name:        "bootnodes",
			Usage:       "bootnode list split by ','",
			Destination: &bootnodes,
		},
	}

	app.Commands = []cli.Command{
		{
			Name:   "attach",
			Usage:  "连接本地已启动的进程",
			Action: AttachCmd,
		},
	}

	app.Before = func(ctx *cli.Context) error {
		return nil
	}
	app.Action = func(ctx *cli.Context) error {
		if homedir == "" {
			panic("homedir can not empty.")
		}
		cfg := alibp2p.Config{
			Ctx:       context.Background(),
			Homedir:   homedir,
			Port:      uint64(port),
			Bootnodes: nil,
			Discover:  !nodiscover,
			Networkid: big.NewInt(int64(networkid)),
		}
		if bootnodes != "" {
			log.Println("bootnodes=", bootnodes)
			cfg.Bootnodes = strings.Split(bootnodes, ",")
		}
		p2pservice = alibp2p.NewService(cfg)
		p2pservice.SetStreamHandler(echopid, func(s network.Stream) {
			log.Println("Got a new stream!")
			if err := func(s network.Stream) error {
				buf := bufio.NewReader(s)
				str, err := buf.ReadString('\n')
				if err != nil {
					return err
				}

				log.Printf("read: %s\n", str)
				_, err = s.Write([]byte(str))
				return err
			}(s); err != nil {
				log.Println(err)
				s.Reset()
			} else {
				s.Close()
			}
		})
		go p2pservice.Start()
		log.Println(">> Action on port =", ctx.GlobalInt("p"))
		rs := &rpcserver.Rpcserver{
			Port:       ctx.GlobalInt("rpcport"),
			ServiceMap: ServiceRegMap,
		}
		rs.StartServer()
		return nil
	}
	app.Run(os.Args)
}

type (
	shellservice struct{}
	cmdFn        func(args ...string) (interface{}, error)
)

//http://localhost:8081/api/?body={"service":"shell","method":"peers"}
func (self *shellservice) Peers(params interface{}) rpcserver.Success {
	peers := p2pservice.Peers()
	return rpcserver.Success{
		Success: true,
		Entity: struct {
			Total int
			Peers []string
		}{len(peers), peers},
	}
}

//http://localhost:8081/api/?body={"service":"shell","method":"echo","params":{"to":"/ip4/127.0.0.1/tcp/10000/ipfs/16Uiu2HAkzfSuviNuR7ez9BMkYw98YWNjyBNNmSLNnoX2XADfZGqP","msg":"hello world"}}
func (self *shellservice) Echo(params interface{}) rpcserver.Success {
	log.Println("echo_params=", params)
	args := params.(map[string]interface{})
	log.Println("echo_args=", args)
	to := args["to"].(string)
	msg := args["msg"].(string)
	s, err := p2pservice.SendMsg(to, echopid, []byte(msg+"\n"))
	rtn := ""
	success := true
	if err != nil {
		rtn = err.Error()
		success = false
	} else {
		buf, err := ioutil.ReadAll(s)
		rtn = string(buf)
		if err != nil {
			s.Reset()
		} else {
			s.Close()
		}
	}
	return rpcserver.Success{
		Success: success,
		Entity:  rtn,
	}
}

func (self *shellservice) Put(params interface{}) rpcserver.Success {
	log.Println("put_params=", params)
	args := params.(map[string]interface{})
	log.Println("put_args=", args)
	k := args["key"].(string)
	v := args["val"].(string)
	success := true
	rtn := ""
	err := p2pservice.Put(k, []byte(v))
	if err != nil {
		success = false
		rtn = err.Error()
	}
	return rpcserver.Success{
		Success: success,
		Entity:  rtn,
	}
}

func (self *shellservice) Get(params interface{}) rpcserver.Success {
	log.Println("get_params=", params)
	args := params.(map[string]interface{})
	log.Println("get_args=", args)
	k := args["key"].(string)
	success := true
	rtn := ""
	b, err := p2pservice.Get(k)
	if err != nil {
		success = false
		rtn = err.Error()
	} else {
		rtn = string(b)
	}
	return rpcserver.Success{
		Success: success,
		Entity:  rtn,
	}
}

func (self *shellservice) Myid(params interface{}) rpcserver.Success {
	return rpcserver.Success{
		Success: true,
		Entity:  p2pservice.Myid(),
	}
}

func AttachCmd(ctx *cli.Context) error {
	<-time.After(time.Second)
	go func() {
		defer close(Stop)
		fmt.Println("------------")
		fmt.Println("hello world")
		fmt.Println("------------")
		for {
			fmt.Print("cmd #>")
			ir := bufio.NewReader(os.Stdin)
			if cmd, err := ir.ReadString('\n'); err == nil && strings.Trim(cmd, " ") != "\n" {
				cmd = strings.Trim(cmd, " ")
				cmd = cmd[:len([]byte(cmd))-1]
				// TODO 用正则表达式拆分指令和参数
				cmdArg := strings.Split(cmd, " ")
				switch cmdArg[0] {
				case "exit", "quit":
					fmt.Println("bye bye ^_^ ")
					return
				default:
					log.Println(cmd)
					if fn, ok := Funcs[cmdArg[0]]; ok {
						if r, err := fn(cmdArg[1:]...); err != nil {
							log.Println(err)
						} else if r != nil {
							fmt.Println(r)
						}
					} else {
						fmt.Println("not support : ", cmdArg[0])
					}
				}
			}
		}
	}()
	<-Stop
	return nil
}

var (
	apiurl  = func(port int) string { return fmt.Sprintf("http://localhost:%d/api/?body=", port) }
	api_put = func(k, v string) string {
		return apiurl(rpcport) + fmt.Sprintf(`{"service":"shell","method":"put","params":{"key":"%s","val":"%s"}}`, k, v)
	}
	api_get = func(k string) string {
		return apiurl(rpcport) + fmt.Sprintf(`{"service":"shell","method":"get","params":{"key":"%s"}}`, k)
	}
	api_peers = func() string {
		return apiurl(rpcport) + `{"service":"shell","method":"peers"}`
	}
	api_myid = func() string {
		return apiurl(rpcport) + `{"service":"shell","method":"myid"}`
	}

	printResp = func(resp *http.Response) {
		success := rpcserver.SuccessFromReader(resp.Body)
		j, _ := json.Marshal(success)
		var out bytes.Buffer
		json.Indent(&out, j, "", "\t")
		out.WriteTo(os.Stdout)
		fmt.Println()
	}
	Funcs = map[string]cmdFn{
		"put": func(args ...string) (interface{}, error) {
			k, v := args[0], args[1]
			resp, err := http.Get(api_put(k, v))
			if err != nil {
				log.Println("error", err)
			}
			if resp != nil && resp.Body != nil {
				defer resp.Body.Close()
			}
			printResp(resp)
			return nil, nil
		},
		"get": func(args ...string) (interface{}, error) {
			k := args[0]
			resp, err := http.Get(api_get(k))
			if err != nil {
				log.Println("error", err)
			}
			if resp != nil && resp.Body != nil {
				defer resp.Body.Close()
			}
			printResp(resp)
			return nil, nil
		},
		"peers": func(args ...string) (interface{}, error) {
			resp, err := http.Get(api_peers())
			if err != nil {
				log.Println("error", err)
			}
			if resp != nil && resp.Body != nil {
				defer resp.Body.Close()
			}
			printResp(resp)
			return nil, nil
		},
		"myid": func(args ...string) (interface{}, error) {
			resp, err := http.Get(api_myid())
			if err != nil {
				log.Println("error", err)
			}
			if resp != nil && resp.Body != nil {
				defer resp.Body.Close()
			}
			printResp(resp)
			return nil, nil
		},
	}
)
