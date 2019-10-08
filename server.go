package alibp2p

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	opts "github.com/libp2p/go-libp2p-kad-dht/opts"
	ma "github.com/multiformats/go-multiaddr"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"path"
	"sync"
)

const (
	ProtocolDHT  protocol.ID = "/pdx/kad/1.0.0"
	NamespaceDHT             = "cc14514"
)

var (
	defBootnodes = []string{
		"/ip4/101.251.230.218/tcp/10000/ipfs/16Uiu2HAkzfSuviNuR7ez9BMkYw98YWNjyBNNmSLNnoX2XADfZGqP",
	}

	DefaultProtocols = []protocol.ID{ProtocolDHT}
	curve            = btcec.S256()
	loadid           = func(homedir string) (crypto.PrivKey, error) {
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

type (
	Service struct {
		ctx       context.Context
		homedir   string
		host      host.Host
		router    routing.Routing
		bootnodes []peer.AddrInfo
		cfg       Config
	}
	blankValidator struct{}
)

func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }

func NewService(cfg Config) *Service {
	log.Println("alibp2p.NewService", cfg)
	priv, err := loadid(cfg.Homedir)
	//hid, _ := peer.IDFromPublicKey(priv.GetPublic())
	//relayaddr, err := ma.NewMultiaddr("/p2p-circuit/ipfs/" + h3.ID().Pretty())
	var router routing.Routing
	listen0, _ := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", cfg.Port))
	//listen1, _ := ma.NewMultiaddr(fmt.Sprintf("/p2p-circuit/ipfs/%s", hid.Pretty()))

	optlist := []libp2p.Option{
		libp2p.ListenAddrs(listen0),
		libp2p.Identity(priv),
		libp2p.EnableAutoRelay(),
		libp2p.EnableRelay(circuit.OptActive, circuit.OptDiscovery, circuit.OptHop),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			if router == nil {
				dht, err := dht.New(cfg.Ctx, h,
					opts.Client(false),
					opts.NamespacedValidator(NamespaceDHT, blankValidator{}),
					opts.Protocols(DefaultProtocols...))
				if err != nil {
					panic(err)
				}
				router = dht
			}
			return router, nil
		}),
	}
	if p, err := cfg.ProtectorOpt(); err == nil {
		optlist = append(optlist, p)
	}

	host, err := libp2p.New(cfg.Ctx, optlist...)

	if err != nil {
		panic(err)
	}

	hostAddr, err := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", host.ID().Pretty()))
	for i, addr := range host.Addrs() {
		full := addr.Encapsulate(hostAddr)
		log.Println(i, "address", full)
	}

	bootpeers := convertPeers(defBootnodes)
	if cfg.Bootnodes != nil && len(cfg.Bootnodes) > 0 {
		bootpeers = convertPeers(cfg.Bootnodes)
	}

	log.Println("New Service end =========>")
	return &Service{
		cfg:       cfg,
		ctx:       cfg.Ctx,
		homedir:   cfg.Homedir,
		host:      host,
		router:    router,
		bootnodes: bootpeers,
	}
}

func (self *Service) SetStreamHandler(pid string, handler func(s network.Stream)) {
	self.host.SetStreamHandler(protocol.ID(pid), handler)
}

func (self *Service) SendMsg(to, protocolID string, msg []byte) (network.Stream, error) {
	ipfsaddr, err := ma.NewMultiaddr(to)
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
	targetPeerAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", peer.IDB58Encode(peerid)))
	relayAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/p2p-circuit/ipfs/%s", peer.IDB58Encode(peerid)))
	targetAddr := ipfsaddr.Decapsulate(targetPeerAddr)
	self.host.Peerstore().AddAddrs(peerid, []ma.Multiaddr{targetAddr, relayAddr}, peerstore.PermanentAddrTTL)
	s, err := self.host.NewStream(context.Background(), peerid, protocol.ID(protocolID))
	if err != nil {
		log.Fatalln(err)
	}
	_, err = s.Write(msg)
	if err != nil {
		log.Fatalln(err)
	}
	return s, err
}

func (self *Service) Start() {
	if self.cfg.Discover {
		self.Bootstrap()
	}
}

func (self *Service) Peers() []string {
	var ret = make([]string, 0)
	for _, c := range self.host.Network().Conns() {
		remoteaddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", c.RemotePeer().Pretty()))
		maddr := c.RemoteMultiaddr().Encapsulate(remoteaddr)
		taddr, _ := maddr.MarshalText()
		ret = append(ret, string(taddr))
	}
	return ret
}

func (self *Service) Put(k string, v []byte) error {
	return self.router.PutValue(self.ctx, fmt.Sprintf("/%s/%s", NamespaceDHT, k), v)
}

func (self *Service) Get(k string) ([]byte, error) {
	return self.router.GetValue(self.ctx, fmt.Sprintf("/%s/%s", NamespaceDHT, k))
}

func (self *Service) Bootstrap() error {
	log.Println("host-addrs", self.host.Addrs())
	log.Println("host-network-listen", self.host.Network().ListenAddresses())
	log.Println("host-peerinfo", self.host.Peerstore().PeerInfo(self.host.ID()))
	err := bootstrapConnect(self.ctx, self.host, self.bootnodes)
	if err != nil {
		panic(err)
	}
	// Bootstrap the host
	err = self.router.Bootstrap(self.ctx)
	if err != nil {
		log.Println("bootstrap-error", "err", err)
		panic(err)
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
			log.Printf("%s bootstrapping to %s : %v", ph.ID(), p.ID, p.Addrs)
			ph.Peerstore().AddAddrs(p.ID, p.Addrs, peerstore.PermanentAddrTTL)
			if err := ph.Connect(ctx, p); err != nil {
				log.Println(ctx, "bootstrapDialFailed", p.ID)
				log.Printf("failed to bootstrap with %v: %s", p.ID, err)
				errs <- err
				return
			}
			log.Println(ctx, "bootstrapDialSuccess", p.ID, ph.Peerstore().PeerInfo(p.ID))
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
