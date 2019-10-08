package main

import (
	"bufio"
	"context"
	"github.com/cc14514/go-alibp2p"
	"github.com/cc14514/go-lightrpc/rpcserver"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/urfave/cli"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"strings"
)

const echopid = "/echo/1.0.0"

var (
	homedir, bootnodes string
	port, networkid    int
	nodiscover         bool
	p2pservice         *alibp2p.Service
	ServiceRegMap      = make(map[string]rpcserver.ServiceReg)
	genServiceReg      = func(namespace, version string, service interface{}) {
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
			Name:  "rpcport",
			Usage: "HTTP-RPC server listening `PORT`",
			Value: 8080,
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
	app.Before = func(ctx *cli.Context) error {
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
		return nil
	}
	app.Action = func(ctx *cli.Context) error {
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

type shellservice struct{}

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
