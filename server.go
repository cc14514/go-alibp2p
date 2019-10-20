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
	pool "github.com/libp2p/go-buffer-pool"
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
	"io"
	"io/ioutil"
	"log"
	"math/big"
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

	connectFn = func(ctx context.Context, ph host.Host, peers []peer.AddrInfo) error {
		if len(peers) < 1 {
			return errors.New("not enough peers to connect")
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
				defer log.Println("connect to", ph.ID(), p.ID)
				log.Printf("%s connecting to %s : %v", ph.ID(), p.ID, p.Addrs)
				ph.Peerstore().AddAddrs(p.ID, p.Addrs, peerstore.PermanentAddrTTL)
				if err := ph.Connect(ctx, p); err != nil {
					log.Println("connect failed", p.ID)
					log.Printf("failed to connect with %v: %s", p.ID, err)
					errs <- err
					return
				}
				log.Println("connect success", p.ID, ph.Peerstore().PeerInfo(p.ID))
				log.Printf("connected with %v", p.ID)
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
			return fmt.Errorf("failed to connect. %s", err)
		}
		return nil
	}

	convertPeers = func(peers []string) ([]peer.AddrInfo, error) {
		pinfos := make([]peer.AddrInfo, len(peers))
		for i, addr := range peers {
			maddr := ma.StringCast(addr)
			p, err := peer.AddrInfoFromP2pAddr(maddr)
			if err != nil {
				log.Println(err)
				return nil, err
			}
			pinfos[i] = *p
		}
		return pinfos, nil
	}

	pubkeyToEcdsa = func(pk crypto.PubKey) *ecdsa.PublicKey {
		k0 := pk.(*crypto.Secp256k1PublicKey)
		pubkey := (*ecdsa.PublicKey)(k0)
		return pubkey
	}
	ecdsaToPubkey = func(pk *ecdsa.PublicKey) crypto.PubKey {
		k0 := (*crypto.Secp256k1PublicKey)(pk)
		return k0
	}

	ECDSAPubEncode = func(pk *ecdsa.PublicKey) (string, error) {
		id, err := peer.IDFromPublicKey(ecdsaToPubkey(pk))
		return id.Pretty(), err
	}
	ECDSAPubDecode = func(pk string) (*ecdsa.PublicKey, error) {
		id, err := peer.IDB58Decode(pk)
		if err != nil {
			return nil, err
		}
		pub, err := id.ExtractPublicKey()
		if err != nil {
			return nil, err
		}
		return pubkeyToEcdsa(pub), nil
	}
)

type (
	Service struct {
		ctx        context.Context
		homedir    string
		host       host.Host
		router     routing.Routing
		bootnodes  []peer.AddrInfo
		cfg        Config
		notifiee   *network.NotifyBundle
		isDirectFn func(id string) bool
	}
	blankValidator struct{}
	ConnType       int
)

const (
	CONNT_TYPE_DIRECT ConnType = iota
	CONN_TYPE_RELAY
	CONN_TYPE_ALL
)

func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }

func NewService(cfg Config) *Service {
	log.Println("alibp2p.NewService", cfg)
	var (
		err       error
		router    routing.Routing
		priv      crypto.PrivKey
		bootnodes []peer.AddrInfo
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
	optlist = append(optlist, libp2p.ConnectionManager(connmgr.NewConnManager(50, 500, time.Second*20)))

	host, err := libp2p.New(cfg.Ctx, optlist...)
	if err != nil {
		panic(err)
	}

	hostAddr, err := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", host.ID().Pretty()))
	for i, addr := range host.Addrs() {
		full := addr.Encapsulate(hostAddr)
		log.Println(i, "address", full)
	}

	if cfg.Bootnodes != nil && len(cfg.Bootnodes) > 0 {
		bootnodes, err = convertPeers(cfg.Bootnodes)
		if err != nil {
			panic(err)
		}
	}

	log.Println("New Service end =========>")
	service := &Service{
		cfg:       cfg,
		ctx:       cfg.Ctx,
		homedir:   cfg.Homedir,
		host:      host,
		router:    router,
		bootnodes: bootnodes,
		notifiee:  new(network.NotifyBundle),
	}

	service.isDirectFn = func(id string) bool {
		direct, _ := service.Conns()
		for _, url := range direct {
			if strings.Contains(url, id) {
				return true
			}
		}
		return false
	}

	return service
}

func (self *Service) ClosePeer(pubkey *ecdsa.PublicKey) error {
	id, err := ECDSAPubEncode(pubkey)
	if err != nil {
		return err
	}
	p, err := peer.IDB58Decode(id)
	if err != nil {
		return err
	}
	return self.host.Network().ClosePeer(p)
}

func (self *Service) SetBootnode(peer ...string) error {
	pi, err := convertPeers(peer)
	self.bootnodes = pi
	return err
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

/*
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
*/

func (self *Service) SetHandler(pid string, handler func(sessionId string, pubkey *ecdsa.PublicKey, rw io.ReadWriter) error) {
	self.host.SetStreamHandler(protocol.ID(pid), func(s network.Stream) {
		defer func() {
			s.Close()
			s.Reset()
		}()
		conn := s.Conn()
		sid := fmt.Sprintf("session:%s%s", conn.RemoteMultiaddr().String(), conn.LocalMultiaddr().String())
		pk, _ := s.Conn().RemotePeer().ExtractPublicKey()
		pubkeyToEcdsa(pk)
		if err := handler(sid, pubkeyToEcdsa(pk), s); err != nil {
			log.Println(err)
		}
	})
}

func (self *Service) SetStreamHandler(protoid string, handler func(s network.Stream)) {
	self.host.SetStreamHandler(protocol.ID(protoid), handler)
}

func (self *Service) SendMsgAfterClose(to, protocolID string, msg []byte) error {
	s, err := self.SendMsg(to, protocolID, msg)
	defer func() {
		if err == nil && s != nil {
			s.Close()
		}
	}()
	return err
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

func (self *Service) OnConnected(t ConnType, callback func(inbound bool, sessionId string, pubKey *ecdsa.PublicKey)) {
	self.notifiee.ConnectedF = func(i network.Network, conn network.Conn) {

		switch t {
		case CONNT_TYPE_DIRECT:
			if !self.isDirectFn(conn.RemotePeer().Pretty()) {
				log.Println("alibp2p-OnConnected skip : want direct :", conn.RemotePeer().Pretty())
				return
			}
		case CONN_TYPE_RELAY:
			if self.isDirectFn(conn.RemotePeer().Pretty()) {
				fmt.Println("alibp2p-OnConnected skip : want relay :", conn.RemotePeer().Pretty())
				return
			}
		case CONN_TYPE_ALL:
		}
		var (
			in    bool
			pk, _ = conn.RemotePeer().ExtractPublicKey()
		)
		switch conn.Stat().Direction {
		case network.DirInbound:
			in = true
		case network.DirOutbound:
			in = false
		}
		/*
			sid := fmt.Sprintf("session:%s%s",
				strings.Split(conn.RemoteMultiaddr().String(), "tcp/")[1],
				strings.Split(conn.RemoteMultiaddr().String(), "tcp/")[1],
			)
		*/
		sid := fmt.Sprintf("session:%s%s", conn.RemoteMultiaddr().String(), conn.LocalMultiaddr().String())
		callback(in, sid, pubkeyToEcdsa(pk))
	}
}

func (self *Service) OnDisconnected(callback func(sessionId string, pubKey *ecdsa.PublicKey)) {
	self.notifiee.DisconnectedF = func(i network.Network, conn network.Conn) {
		pk, _ := conn.RemotePeer().ExtractPublicKey()
		for _, c := range i.Conns() {
			c.RemotePeer()
		}
		sid := fmt.Sprintf("session:%s%s", conn.RemoteMultiaddr().String(), conn.LocalMultiaddr().String())
		callback(sid, pubkeyToEcdsa(pk))
	}
}

func (self *Service) Start() {
	self.host.Network().Notify(self.notifiee)
	if self.cfg.Discover {
		self.bootstrap()
	}
}

func (self *Service) Table() map[string][]string {
	r := make(map[string][]string, 0)
	for _, p := range self.host.Peerstore().Peers() {
		a := make([]string, 0)
		pi := self.host.Peerstore().PeerInfo(p)
		for _, addr := range pi.Addrs {
			a = append(a, addr.String())
		}
		r[p.Pretty()] = a
	}
	return r
}

func (self *Service) Conns() (direct []string, relay []string) {
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

func (self *Service) Peers() (direct []string, relay map[string][]string, total int) {
	direct, relay, total = make([]string, 0), make(map[string][]string), 0
	dl, rl := self.Conns()
	for _, d := range dl {
		direct = append(direct, strings.Split(d, "/ipfs/")[1])
		total += 1
	}
	for _, r := range rl {
		arr := strings.Split(r, "/p2p-circuit")
		f, t := arr[0], arr[1]

		rarr, ok := relay[strings.Split(f, "/ipfs/")[1]]
		if !ok {
			rarr = make([]string, 0)
		}
		rarr = append(rarr, strings.Split(t, "/ipfs/")[1])
		relay[strings.Split(f, "/ipfs/")[1]] = rarr
		total += 1
	}
	return direct, relay, total
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

func (self *Service) BootstrapOnce() error {
	err := self.router.(*dht.IpfsDHT).BootstrapOnce(self.ctx, dht.DefaultBootstrapConfig)
	if err != nil {
		log.Println("bootstrap-error", "err", err)
	}
	return err
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
					if self.bootnodes != nil && len(self.host.Network().Conns()) < len(self.bootnodes) {
						// 在 peerstore 中随机找至多 5 个节点尝试连接
						var (
							limit  = 3
							others = self.peersWithoutBootnodes()
							total  = len(others)
						)
						log.Println("bootstrap looping")
						err := connectFn(self.ctx, self.host, self.bootnodes)
						if err == nil {
							log.Println("bootstrap success")
							if atomic.CompareAndSwapInt32(&loopbootstrap, 0, 1) {
								log.Println("Bootstrap the host")
								err = self.router.Bootstrap(self.ctx)
								if err != nil {
									log.Println("bootstrap-error", "err", err)
								}
							} else {
								log.Println("Reconnected and bootstrap the host once")
								self.BootstrapOnce()
							}
						} else if total > 0 {
							if total < limit {
								limit = total
							}
							tasks := randPeers(others, limit)
							err := connectFn(self.ctx, self.host, tasks)
							log.Println("bootstrap fail try to conn others -->", "err", err, "total", total, "limit", limit, "tasks", tasks)
						}
					}
				}
			}
		}
		log.Println("loopboot-end")
	}()
	return nil
}

func randPeers(others []peer.AddrInfo, limit int) []peer.AddrInfo {
	_, randk, _ := crypto.GenerateRSAKeyPair(1024, rand.Reader)
	rnBytes, _ := randk.Bytes()
	n := new(big.Int).Mod(new(big.Int).SetBytes(rnBytes), big.NewInt(int64(len(others)))).Int64()
	others = append(others[n:], others[:n]...)
	others = others[:limit]
	return others
}

func (self *Service) peersWithoutBootnodes() []peer.AddrInfo {
	var (
		result  = make([]peer.AddrInfo, 0)
		bootmap = make(map[string]interface{})
	)
	for _, b := range self.bootnodes {
		bootmap[b.ID.Pretty()] = struct{}{}
	}

	for _, p := range self.host.Peerstore().Peers() {
		if _, ok := bootmap[p.Pretty()]; ok {
			continue
		}
		if p.Pretty() == self.host.ID().Pretty() {
			continue
		}
		if pi := self.host.Peerstore().PeerInfo(p); pi.Addrs != nil && len(pi.Addrs) > 0 {
			result = append(result, pi)
		}
	}

	return result
}

func GetBuf(size int) []byte {
	return pool.Get(size)
}
func PutBuf(buf []byte) {
	pool.Put(buf)
}
