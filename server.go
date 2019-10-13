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
	connmgr "github.com/libp2p/go-libp2p-connmgr"
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
	msmux "github.com/multiformats/go-multistream"

	"io"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	ProtocolDHT  protocol.ID = "/pdx/kad/1.0.0"
	NamespaceDHT             = "cc14514"
)

var (
	loopboot, loopbootstrap int32
	defBootnodes            = []string{
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
		notifiee  *network.NotifyBundle
	}
	blankValidator struct{}

	SConn struct {
		network.Stream
	}
)

func (S SConn) LocalAddr() net.Addr {
	return S.LocalAddr()
}

func (S SConn) RemoteAddr() net.Addr {
	return S.RemoteAddr()
}

func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }

func NewService(cfg Config) *Service {
	log.Println("alibp2p.NewService", cfg)
	var (
		err    error
		router routing.Routing
		priv   crypto.PrivKey
	)
	if cfg.PrivKey != nil {
		_p := (*crypto.Secp256k1PrivateKey)(cfg.PrivKey)
		priv = (crypto.PrivKey)(_p)
	} else {
		priv, err = loadid(cfg.Homedir)
	}
	//hid, _ := peer.IDFromPublicKey(priv.GetPublic())
	//relayaddr, err := ma.NewMultiaddr("/p2p-circuit/ipfs/" + h3.ID().Pretty())
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
	optlist = append(optlist, libp2p.ConnectionManager(connmgr.NewConnManager(600, 900, time.Second*20)))

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
		notifiee:  new(network.NotifyBundle),
	}
}

func (self *Service) Myid() map[string]interface{} {
	addrs := make([]string, 0)
	for _, maddr := range self.host.Addrs() {
		addrs = append(addrs, maddr.String())
	}
	return map[string]interface{}{
		"Id":    self.host.ID().Pretty(),
		"Addrs": addrs,
	}
}

func (self *Service) GetConn(pid string) chan SConn {
	connCh := make(chan SConn)
	self.host.Mux().AddHandler(pid, func(p string, rwc io.ReadWriteCloser) error {
		is := rwc.(network.Stream)
		is.SetProtocol(protocol.ID(p))
		fmt.Println("func (self *Service) GetConn(pid string) chan SConn 11111")
		connCh <- SConn{Stream: is}
		fmt.Println("func (self *Service) GetConn(pid string) chan SConn 22222")
		return nil
	})
	return connCh
}

func (self *Service) SetHandler(pid string, handler func(peerid string, rw io.ReadWriter) error) {
	self.host.SetStreamHandler(protocol.ID(pid), func(s network.Stream) {
		if err := handler(s.Conn().RemotePeer().Pretty(), s); err != nil {
			log.Println(err)
			s.Reset()
		}
	})
}

func (self *Service) SetStreamHandler(pid string, handler func(s network.Stream)) {
	self.host.SetStreamHandler(protocol.ID(pid), handler)
}

func (self *Service) SendMsg(to, protocolID string, msg []byte) (network.Stream, error) {
	peerid, err := peer.IDB58Decode(to)
	if err != nil {
		ipfsaddr, err := ma.NewMultiaddr(to)
		if err != nil {
			log.Println(err)
			return nil, err
		}

		pid, err := ipfsaddr.ValueForProtocol(ma.P_IPFS)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		peerid, err = peer.IDB58Decode(pid)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		targetPeerAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", peer.IDB58Encode(peerid)))
		targetAddr := ipfsaddr.Decapsulate(targetPeerAddr)
		relayAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/p2p-circuit/ipfs/%s", peer.IDB58Encode(peerid)))
		raddr := []ma.Multiaddr{relayAddr, targetAddr}
		self.host.Peerstore().AddAddrs(peerid, raddr, peerstore.PermanentAddrTTL)
	} else {
		pi, err := self.router.FindPeer(self.ctx, peerid)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		self.host.Peerstore().AddAddrs(pi.ID, pi.Addrs, peerstore.PermanentAddrTTL)
	}

	s, err := self.host.NewStream(context.Background(), peerid, protocol.ID(protocolID))
	defer func() {
		if err != nil && s != nil {
			s.Reset()
		} else if s != nil {
			s.Close()
		}
	}()
	if err != nil {
		log.Println(err)
		return nil, err
	}
	_, err = s.Write(msg)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return s, err
}

func (self *Service) OnConnected(ctx context.Context, protoid string, callback func(pubKey *ecdsa.PublicKey, fd net.Conn)) {
	self.host.Network().SetConnHandler(func(conn network.Conn) {
		connCh := self.GetConn(protoid)
		pk, err := conn.RemotePeer().ExtractPublicKey()
		if err != nil {
			fmt.Println("---- Error 1--->", err)
			return
		}
		k0 := pk.(*crypto.Secp256k1PublicKey)
		pubkey := (*ecdsa.PublicKey)(k0)

		sconn, err := conn.NewStream()
		fmt.Println("---- Error 2--->", err)
		sconn.SetProtocol(protocol.ID(protoid))
		// ok give the response to our handler.
		if err := msmux.SelectProtoOrFail(protoid, sconn); err != nil {
			fmt.Println("---- Error 3--->", err)
			sconn.Reset()
			return
		}

		sconn.Write([]byte("open"))
		fd := <-connCh
		callback(pubkey, fd)
	})
}

func (self *Service) OnDisconnected(callback func(pubKey *ecdsa.PublicKey)) {
	self.notifiee.DisconnectedF = func(i network.Network, conn network.Conn) {
		pk, err := conn.RemotePeer().ExtractPublicKey()
		if err != nil {
			fmt.Println("---- Error --->", err)
			fmt.Println(err)
			return
		}
		k0 := pk.(*crypto.Secp256k1PublicKey)
		pubkey := (*ecdsa.PublicKey)(k0)
		callback(pubkey)
	}
}

func (self *Service) Start() {
	self.host.Network().Notify(self.notifiee)
	if self.cfg.Discover {
		self.bootstrap()
	}
}

func (self *Service) Peers() (direct []string, relay []string) {
	direct, relay = make([]string, 0), make([]string, 0)
	for _, c := range self.host.Network().Conns() {
		remoteaddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", c.RemotePeer().Pretty()))
		maddr := c.RemoteMultiaddr().Encapsulate(remoteaddr)
		taddr, _ := maddr.MarshalText()
		if strings.Contains(string(taddr), "p2p-circuit") {
			relay = append(relay, string(taddr))
		} else {
			direct = append(direct, string(taddr))
		}
	}
	return direct, relay
}

func (self *Service) PutPeerMeta(id, key string, v interface{}) error {
	p, err := peer.IDB58Decode(id)
	if err != nil {
		return err
	}
	return self.host.Peerstore().Put(p, key, v)
}

func (self *Service) GetPeerMeta(id, key string) (interface{}, error) {
	p, err := peer.IDB58Decode(id)
	if err != nil {
		return nil, err
	}
	return self.host.Peerstore().Get(p, key)
}

func (self *Service) Findpeer(id string) (peer.AddrInfo, error) {
	peerid, err := peer.IDB58Decode(id)
	if err != nil {
		return peer.AddrInfo{}, err
	}
	return self.router.FindPeer(self.ctx, peerid)
}

func (self *Service) Put(k string, v []byte) error {
	return self.router.PutValue(self.ctx, fmt.Sprintf("/%s/%s", NamespaceDHT, k), v)
}

func (self *Service) Get(k string) ([]byte, error) {
	return self.router.GetValue(self.ctx, fmt.Sprintf("/%s/%s", NamespaceDHT, k))
}

func (self *Service) bootstrap() error {
	log.Println("host-addrs", self.host.Addrs())
	log.Println("host-network-listen", self.host.Network().ListenAddresses())
	log.Println("host-peerinfo", self.host.Peerstore().PeerInfo(self.host.ID()))
	go func() {
		log.Println("loopboot-start")
		if atomic.CompareAndSwapInt32(&loopboot, 0, 1) {
			defer func() {
				atomic.StoreInt32(&loopboot, 0)
				atomic.StoreInt32(&loopbootstrap, 0)
			}()
			for {
				select {
				case <-self.ctx.Done():
					return
				case <-time.After(5 * time.Second):
					if len(self.host.Network().Conns()) < len(self.bootnodes) {
						err := bootstrapConnect(self.ctx, self.host, self.bootnodes)
						if err == nil {
							if atomic.CompareAndSwapInt32(&loopbootstrap, 0, 1) {
								log.Println("Bootstrap the host")
								err = self.router.Bootstrap(self.ctx)
								if err != nil {
									log.Println("bootstrap-error", "err", err)
								}
							} else {
								log.Println("Reconnected and bootstrap the host once")
								err = self.router.(*dht.IpfsDHT).BootstrapOnce(self.ctx, dht.DefaultBootstrapConfig)
								if err != nil {
									log.Println("bootstrap-error", "err", err)
								}
							}
						}
					}
				}
			}
		}
		log.Println("loopboot-end")
	}()
	return nil
}

func convertPeers(peers []string) []peer.AddrInfo {
	pinfos := make([]peer.AddrInfo, len(peers))
	for i, addr := range peers {
		maddr := ma.StringCast(addr)
		p, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			log.Println(err)
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
