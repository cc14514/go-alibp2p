package alibp2p

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/btcsuite/btcd/btcec"
	"github.com/libp2p/go-libp2p"
	circuit "github.com/libp2p/go-libp2p-circuit"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/protocol"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	opts "github.com/libp2p/go-libp2p-kad-dht/opts"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	ma "github.com/multiformats/go-multiaddr"
	"os"
	"sync"
	"time"

	"io/ioutil"
	"log"
	"math/big"
	"path"

	"errors"
)

var (
	defBootnodes = []string{
		//"/ip4/127.0.0.1/tcp/10000/ipfs/16Uiu2HAkzfSuviNuR7ez9BMkYw98YWNjyBNNmSLNnoX2XADfZGqP",
		"/ip4/101.251.230.218/tcp/10000/ipfs/16Uiu2HAkzfSuviNuR7ez9BMkYw98YWNjyBNNmSLNnoX2XADfZGqP",
	}
	ProtocolDHT      protocol.ID = "/pdx/kad/1.0.0"
	DefaultProtocols             = []protocol.ID{ProtocolDHT}
	curve                        = btcec.S256()
	loadid                       = func(homedir string) (crypto.PrivKey, error) {
		keypath := path.Join(homedir, "p2p.id")
		log.Println("keypath", keypath)
		buff1, err := ioutil.ReadFile(keypath)
		if err != nil {
			err := os.MkdirAll(homedir, 0755)
			if err != nil {
				panic(err)
			}
			priv, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
			k0 := priv.(*crypto.Secp256k1PrivateKey)
			k1 := (*ecdsa.PrivateKey)(k0)
			arr := [3]*big.Int{k1.X, k1.Y, k1.D}
			buff, _ := json.Marshal(arr)
			s0 := hex.EncodeToString(buff)
			err = ioutil.WriteFile(keypath, []byte(s0), 0755)
			if err != nil {
				panic(err)
			}
			return priv, err
		}
		var arr1 = new([3]*big.Int)
		buff, _ := hex.DecodeString(string(buff1))
		err = json.Unmarshal(buff, arr1)
		if err != nil {
			panic(err)
		}
		priv1 := new(ecdsa.PrivateKey)
		priv1.X, priv1.Y, priv1.D, priv1.Curve = arr1[0], arr1[1], arr1[2], curve
		priv2 := (*crypto.Secp256k1PrivateKey)(priv1)
		priv3 := (crypto.PrivKey)(priv2)
		return priv3, err
	}
)

type Service struct {
	ctx       context.Context
	homedir   string
	host      host.Host
	router    *dht.IpfsDHT
	bootnodes []peer.AddrInfo
}

func NewService(ctx context.Context, homedir string, port int, bootnodes []string) *Service {
	log.Println("alibp2p.NewService", "homedir", homedir, "port", port)
	priv, err := loadid(homedir)
	host, err := libp2p.New(ctx,
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port)),
		libp2p.Identity(priv),
		libp2p.EnableRelay(circuit.OptActive, circuit.OptDiscovery, circuit.OptHop))
	if err != nil {
		panic(err)
	}
	hostAddr, err := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", host.ID().Pretty()))
	for i, addr := range host.Addrs() {
		full := addr.Encapsulate(hostAddr)
		log.Println(i, "address", full)
	}
	dht, err := dht.New(ctx, host,
		opts.Client(false),
		opts.Protocols(DefaultProtocols...))
	if err != nil {
		log.Println("dht-error", "err", err)
		panic(err)
	}

	routedHost := rhost.Wrap(host, dht)

	host.SetStreamHandler("/echo/1.0.0", func(s network.Stream) {
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

	bootpeers := convertPeers(defBootnodes)
	if bootnodes != nil && len(bootnodes) > 0 {
		bootpeers = convertPeers(bootnodes)
	}

	return &Service{
		ctx:       ctx,
		homedir:   homedir,
		host:      routedHost,
		router:    dht,
		bootnodes: bootpeers,
	}
}

func (self *Service) Connect(target string) error {

	ipfsaddr, err := ma.NewMultiaddr(target)
	if err != nil {
		log.Fatalln(err)
	}

	pid, err := ipfsaddr.ValueForProtocol(ma.P_IPFS)
	if err != nil {
		log.Fatalln(err)
	}

	peerid, err := peer.IDB58Decode(pid)
	if err != nil {
		log.Fatalln(err)
	}

	// Decapsulate the /ipfs/<peerID> part from the target
	// /ip4/<a.b.c.d>/ipfs/<peer> becomes /ip4/<a.b.c.d>
	targetPeerAddr, _ := ma.NewMultiaddr(
		fmt.Sprintf("/ipfs/%s", peer.IDB58Encode(peerid)))
	targetAddr := ipfsaddr.Decapsulate(targetPeerAddr)
	// We have a peer ID and a targetAddr so we add it to the peerstore
	// so LibP2P knows how to contact it
	self.host.Peerstore().AddAddr(peerid, targetAddr, peerstore.PermanentAddrTTL)
	log.Println("opening stream")
	// make a new stream from host B to host A
	// it should be handled on host A by the handler we set above because
	// we use the same /echo/1.0.0 protocol
	s, err := self.host.NewStream(context.Background(), peerid, "/echo/1.0.0")
	if err != nil {
		log.Fatalln(err)
	}

	_, err = s.Write([]byte("Hello, world!\n"))
	if err != nil {
		log.Fatalln(err)
	}

	out, err := ioutil.ReadAll(s)
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("read reply: %q\n", out)
	return nil
}
func (self *Service) Start() {
	err := bootstrapConnect(self.ctx, self.host, self.bootnodes)
	if err != nil {
		panic(err)
	}
	// Bootstrap the host
	err = self.router.Bootstrap(self.ctx)
	if err != nil {
		panic(err)
	}

	// TODO 连接 bootnode peer 并且启动 discover bootstrap
	tick := time.NewTicker(5 * time.Second)
	for range tick.C {
		fmt.Println(len(self.host.Network().Conns()), "--------------------------------->")
		for j, c := range self.host.Network().Conns() {
			//localaddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", c.LocalPeer().Pretty()))
			//remoteaddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", c.RemotePeer().Pretty()))
			//fmt.Println("=>", j, c.LocalMultiaddr().Encapsulate(localaddr), "->", c.RemoteMultiaddr().Encapsulate(remoteaddr))
			remoteaddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", c.RemotePeer().Pretty()))
			fmt.Println("peers >", j, c.RemoteMultiaddr().Encapsulate(remoteaddr))
		}
	}
}

func (self *Service) Bootstrap() error {
	err := self.router.Bootstrap(self.ctx)
	if err != nil {
		log.Println("bootstrap-error", "err", err)
	}
	return err
}

func convertPeers(peers []string) []peer.AddrInfo {
	pinfos := make([]peer.AddrInfo, len(peers))
	for i, addr := range peers {
		maddr := ma.StringCast(addr)
		p, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			log.Fatalln(err)
		}
		pinfos[i] = *p
	}
	return pinfos
}

// This code is borrowed from the go-ipfs bootstrap process
func bootstrapConnect(ctx context.Context, ph host.Host, peers []peer.AddrInfo) error {
	if len(peers) < 1 {
		return errors.New("not enough bootstrap peers")
	}

	errs := make(chan error, len(peers))
	var wg sync.WaitGroup
	for _, p := range peers {
		if ph.ID() == p.ID {
			continue
		}
		wg.Add(1)
		go func(p peer.AddrInfo) {
			defer wg.Done()
			defer log.Println(ctx, "bootstrapDial", ph.ID(), p.ID)
			log.Printf("%s bootstrapping to %s", ph.ID(), p.ID)

			ph.Peerstore().AddAddrs(p.ID, p.Addrs, peerstore.PermanentAddrTTL)
			if err := ph.Connect(ctx, p); err != nil {
				log.Println(ctx, "bootstrapDialFailed", p.ID)
				log.Printf("failed to bootstrap with %v: %s", p.ID, err)
				errs <- err
				return
			}
			log.Println(ctx, "bootstrapDialSuccess", p.ID)
			log.Printf("bootstrapped with %v", p.ID)
		}(p)
	}
	wg.Wait()
	close(errs)
	count := 0
	var err error
	for err = range errs {
		if err != nil {
			count++
		}
	}
	if count == len(peers) {
		return fmt.Errorf("failed to bootstrap. %s", err)
	}
	return nil
}
