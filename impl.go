package alibp2p

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"github.com/ipfs/go-cid"
	golog "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	circuit "github.com/libp2p/go-libp2p-circuit"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	discoveryopt "github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/helpers"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-core/routing"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	"io/ioutil"
	"math/big"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

var log = golog.Logger("alibp2p")

func New(opts ...Option) (Alibp2pService, Config, error) {
	opts = append(opts, FallbackDefaults)
	var cfg Config
	if err := cfg.Apply(opts...); err != nil {
		return nil, cfg, err
	}
	return newService(cfg), cfg, nil
}

// Deprecated: Use New.
func NewService(cfg Config) Alibp2pService {
	return newService(cfg)
}

func newService(cfg Config) Alibp2pService {
	switch cfg.Loglevel {
	case 5:
		golog.SetAllLoggers(golog.LevelDebug)
	case 3, 4:
		golog.SetAllLoggers(golog.LevelInfo)
	case 0, 1, 2:
		golog.SetAllLoggers(golog.LevelWarn)
	default:
		golog.SetAllLoggers(golog.LevelError)
	}
	log.Debug("alibp2p-service::alibp2p.NewService", cfg)

	var (
		err             error
		router          routing.Routing
		priv            crypto.PrivKey
		bootnodes       []peer.AddrInfo
		connLow, connHi = cfg.ConnLow, cfg.ConnHi
		ps              = pstoremem.NewPeerstore()
	)
	if cfg.MaxMsgSize > 1024*1024 {
		def_maxsize = cfg.MaxMsgSize
	}
	if connLow == 0 {
		connLow = defConnLow
	}
	if connHi == 0 {
		connHi = defConnHi
	}
	if cfg.PrivKey != nil {
		_p := (*crypto.Secp256k1PrivateKey)(cfg.PrivKey)
		priv = (crypto.PrivKey)(_p)
	} else {
		priv, err = loadid(cfg.Homedir)
		cfg.PrivKey = (*ecdsa.PrivateKey)((priv).(*crypto.Secp256k1PrivateKey))
	}
	list := make([]ma.Multiaddr, 0)
	listen0, _ := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", cfg.Port))
	list = append(list, listen0)
	if cfg.MuxPort != nil && cfg.MuxPort.Int64() > 0 {
		listen1, _ := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/mux/%d:%d", cfg.MuxPort, cfg.Port))
		list = append(list, listen1)
		//netmux.Register(cfg.Ctx, int(cfg.MuxPort.Int64()), int(cfg.Port))
	}

	bwc := metrics.NewBandwidthCounter()
	msgc := metrics.NewBandwidthCounter()
	var mo = dht.ModeServer
	if cfg.DisableInbound {
		mo = dht.ModeClient
		//DefaultProtocols = append(DefaultProtocols, ProtocolPlume)
	}
	optlist := []libp2p.Option{
		libp2p.Peerstore(ps),
		libp2p.BandwidthReporter(bwc),
		libp2p.NATPortMap(),
		libp2p.ListenAddrs(list...),
		libp2p.Identity(priv),
		libp2p.ConnectionManager(connmgr.NewConnManager(int(connLow), int(connHi), time.Second*30)),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			if router == nil {
				dht, err := dht.New(cfg.Ctx, h,
					dht.Metrics(cfg.EnableMetric),
					dht.Mode(mo),
					/*
						dht.Validator(record.NamespacedValidator{
							"pk":   record.PublicKeyValidator{},
							"ipns": ipns.Validator{KeyBook: ps},
						}),
					*/
					dht.ProtocolPrefix("alibp2p"),
					dht.NamespacedValidator(NamespaceDHT, blankValidator{}),
				)
				if err != nil {
					panic(fmt.Errorf("dht : %v", err))
				}
				router = dht
			}
			return router, nil
		}),
	}

	optlist = append(optlist, libp2p.EnableAutoRelay())
	if cfg.Relay {
		//os.Setenv("alibp2prelay", "enable") // 在这里使用 go-libp2p/p2p/protocol/identify/id.go:225
		optlist = append(optlist, libp2p.EnableRelay(circuit.OptActive, circuit.OptHop), libp2p.EnableNATService())
	} else {
		optlist = append(optlist, libp2p.EnableRelay(circuit.OptActive))
	}

	if p, err := cfg.ProtectorOpt(); err == nil {
		optlist = append(optlist, p)
	}
	optlist = append(optlist, cfg.MuxTransportOption(cfg.Loglevel))

	host, err := libp2p.New(cfg.Ctx, optlist...)
	if err != nil {
		panic(err)
	}

	_ps, err := pubsub.NewGossipSub(context.Background(), host, pubsub.WithDiscovery(discovery.NewRoutingDiscovery(router)))
	if err != nil {
		panic(err)
	}

	hostAddr, err := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", host.ID().Pretty()))
	for i, addr := range host.Addrs() {
		full := addr.Encapsulate(hostAddr)
		log.Infof("[%d] listen on %v", i, full)
	}

	if cfg.Bootnodes != nil && len(cfg.Bootnodes) > 0 {
		bootnodes, err = convertPeers(cfg.Bootnodes)
		if err != nil {
			panic(err)
		}
	}

	service := &Service{
		cfg:              cfg,
		ctx:              cfg.Ctx,
		ps:               _ps,
		homedir:          cfg.Homedir,
		host:             host,
		router:           router,
		routingDiscovery: discovery.NewRoutingDiscovery(router),
		bootnodes:        bootnodes,
		notifiee:         make([]*network.NotifyBundle, 0),
		bwc:              bwc,
		msgc:             msgc,
		nsttl:            make(map[string]time.Duration),
		clientProtocols:  make(map[string]struct{}),
		topics:           make(map[string]*pubsubObj),
	}

	if cfg.ClientProtocols != nil {
		for _, p := range cfg.ClientProtocols {
			service.clientProtocols[p] = struct{}{}
		}
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

	service.asc = NewAStreamCatch(msgc)
	service.OnDisconnected(func(sessionId string, pubKey *ecdsa.PublicKey) {
		id, _ := ECDSAPubEncode(pubKey)
		service.asc.del2(id, "", "")
	})

	if cfg.DisableInbound {
		service.OnConnected(CONN_TYPE_ALL, nil, func(inbound bool, sessionId string, pubKey *ecdsa.PublicKey, _ []byte) {
			if inbound {
				id, _ := ECDSAPubEncode(pubKey)
				log.Warnf("Node Reject Inbound : %s , %s", id, sessionId)
				service.ClosePeer(pubKey)
			}
		})
	}

	return service
}

func (service *Service) ClosePeer(pubkey *ecdsa.PublicKey) error {
	id, err := ECDSAPubEncode(pubkey)
	if err != nil {
		return err
	}
	p, err := peer.Decode(id)
	if err != nil {
		return err
	}
	return service.host.Network().ClosePeer(p)
}

func (service *Service) SetBootnode(peer ...string) error {
	pi, err := convertPeers(peer)
	service.bootnodes = pi
	return err
}

func (service *Service) Myid() (id string, addrs []string) {
	id = service.host.ID().Pretty()
	addrs = make([]string, 0)
	for _, maddr := range service.host.Addrs() {
		if a := maddr.String(); !strings.Contains(a, "/p2p-circuit") {
			addrs = append(addrs, maddr.String())
		}
	}
	return
}

func (service *Service) SetHandler(pid string, handler StreamHandler) {
	service.checkReuse(pid)
	service.host.SetStreamHandler(protocol.ID(pid), func(s network.Stream) { service.callHandler(s, handler) })
}

func (service *Service) SetHandlerReuseStream(pid string, handler StreamHandler) {
	service.asc.regist(pid, handler)
	service.host.SetStreamHandler(protocol.ID(pid), service.asc.handleStream)
}

func (service *Service) callHandler(s network.Stream, fn interface{}) {
	if fn == nil {
		return
	}
	service.msgc.LogRecvMessage(1)
	service.msgc.LogRecvMessageStream(1, s.Protocol(), s.Conn().RemotePeer())
	defer func() {
		if s != nil {
			go helpers.FullClose(s)
		}
	}()
	conn := s.Conn()
	sid := fmt.Sprintf("session:%s%s", conn.RemoteMultiaddr().String(), conn.LocalMultiaddr().String())
	pk, err := id2pubkey(s.Conn().RemotePeer())
	if err != nil {
		log.Error(err)
		return
	}
	pubkeyToEcdsa(pk)
	if handler, ok := fn.(StreamHandler); ok {
		if err := handler(sid, pubkeyToEcdsa(pk), s); err != nil {
			log.Error(err)
		}
	} else if handler, ok := fn.(StreamHandlerWithProtocol); ok {
		if err := handler(sid, string(s.Protocol()), pubkeyToEcdsa(pk), s); err != nil {
			log.Error(err)
		}
	} else {
		panic(unknow_stream_handler_type)
	}
}

func (service *Service) SetHandlerWithProtocol(pid string, handler StreamHandlerWithProtocol) {
	service.checkReuse(pid)
	service.host.SetStreamHandler(protocol.ID(pid), func(s network.Stream) { service.callHandler(s, handler) })
}

func (service *Service) SetHandlerReuseStreamWithProtocol(pid string, handler StreamHandlerWithProtocol) {
	service.asc.regist(pid, handler)
	service.host.SetStreamHandler(protocol.ID(pid), service.asc.handleStream)
}

func (service *Service) checkReuse(pid string) {
	if service.asc.has(pid) {
		panic("ReuseStream model just provid : SetHandlerReuseStream(string,ReuseStreamHandler)")
	}
}

func (service *Service) SetStreamHandler(protoid string, handler func(s network.Stream)) {
	service.checkReuse(protoid)
	service.host.SetStreamHandler(protocol.ID(protoid), func(a network.Stream) {
		if handler == nil {
			return
		}
		if a != nil {
			service.msgc.LogRecvMessage(1)
			service.msgc.LogRecvMessageStream(1, a.Protocol(), a.Conn().RemotePeer())
		}
		handler(a)
	})
}

//TODO add by liangc : connMgr protected / unprotected setting
func (service *Service) SendMsgAfterClose(to, protocolID string, msg []byte) error {
	if service.asc.has(protocolID) {
		log.Debug("alibp2p::SendMsgAfterClose-lock:try", "id", to, "protocolID", protocolID)
		if err := service.asc.takelock(to, protocolID); err != nil {
			log.Error("alibp2p::SendMsgAfterClose-lock:fail", "id", to, "protocolID", protocolID, "err", err.Error())
			return err
		}
		log.Debug("alibp2p::SendMsgAfterClose-lock:success", "id", to, "protocolID", protocolID)
		defer service.asc.unlock(to, protocolID)
	}
	id, s, _, err := service.sendMsg(to, protocolID, msg, notimeout)
	//service.host.ConnManager().Protect(id, "tmp")
	if err != nil {
		log.Errorf("alibp2p::SendMsgAfterClose-error-1 id=%s , protocolID=%s , err=%v", id, protocolID, err.Error())
		//service.host.Network().ClosePeer(id)
		return err
	}
	if s != nil && !service.asc.has(protocolID) {
		go helpers.FullClose(s)
	} else {
		// reuse channel
		rsp := new(RawData)
		// TODO
		//s.SetReadDeadline(time.Now().Add(30 * time.Second))
		_, err := FromReader(s, rsp)
		if err != nil {
			log.Errorf("alibp2p::SendMsgAfterClose-error-2 id=%s , protocolID=%s , err=%v", id, protocolID, err.Error())
			service.asc.del2(to, protocolID, "")
			return err
		}
		if rsp.Err != "" {
			service.asc.del2(to, protocolID, "")
			log.Errorf("alibp2p::SendMsgAfterClose-error-3 id=%s , protocolID=%s , err=%v", id, protocolID, rsp.Err)
			return errors.New(rsp.Err)
		}
		if s != nil {
			s.SetReadDeadline(notimeout)
		}
		log.Debugf("alibp2p::SendMsgAfterClose-ack %s@%s msgid=%d", protocolID, to, rsp.ID())
	}
	//service.host.ConnManager().Unprotect(id, "tmp")
	return nil
}

func (service *Service) Connect(url string) error {
	ipfsaddr, err := ma.NewMultiaddr(url)
	if err != nil {
		return err
	}
	pid, err := ipfsaddr.ValueForProtocol(ma.P_IPFS)
	if err != nil {
		return err
	}
	peerid, err := peer.Decode(pid)
	if err != nil {
		return err
	}
	targetPeerAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", peer.IDB58Encode(peerid)))
	targetAddr := ipfsaddr.Decapsulate(targetPeerAddr)
	return service.host.Connect(service.ctx, peer.AddrInfo{ID: peerid, Addrs: []ma.Multiaddr{targetAddr}})
}

// 设置 Advertised 的 namespace 生命周期，这个值需要提前设置，以便 Providers 按照设置来提供心跳
// ttl 单位为秒
func (service *Service) getAdvertiseTTL(ns string) time.Duration {
	if ttl := service.nsttl[ns]; ttl > 0 {
		return ttl
	}
	return def_nsttl
}

func (service *Service) SetAdvertiseTTL(ns string, ttl time.Duration) {
	service.nsttl[ns] = ttl
	nsk, _ := nsToCid(ns)
	nsv := big.NewInt(int64(ttl)).String()
	hk, _ := mh.Cast(nsk.Hash())
	os.Setenv(hk.String(), nsv)
	log.Info("SetAdvertiseTTL", "ns", ns, "nsk", nsk, "nsv", nsv)
}

func (service *Service) Advertise(ctx context.Context, ns string) {
	/*
		这里使用了 DHT 实现的接口
		代码：github.com/cc14514/go-libp2p-kad-dht/providers/providers.go
		默认 GC 规则 :
			24 小时清理一次， Advertise 会每隔 3 小时汇报一次，以免被 GC
			...
				case gcTime.Sub(t) > ProvideValidity: // ProvideValidity = 24H , 意思是 key 的 有效期是 24 小时
				// or expired
				err = pm.dstore.Delete(ds.RawKey(res.Key))
				if err != nil && err != ds.ErrNotFound {
					log.Warning("failed to remove provider record from disk: ", err)
				}
			...
	*/
	ttl := service.getAdvertiseTTL(ns)
	log.Infof("Advertise-ttl : %v", ttl)
	discovery.Advertise(ctx, service.routingDiscovery, ns, discoveryopt.TTL(ttl))
}

func nsToCid(ns string) (cid.Cid, error) {
	h, err := mh.Sum([]byte(ns), mh.SHA2_256, -1)
	if err != nil {
		return cid.Undef, err
	}

	return cid.NewCidV1(cid.Raw, h), nil
}

func (service *Service) FindProviders(ctx context.Context, ns string, limit int) ([]string, error) {
	var (
		err error
		ret = make([]string, 0)
		aCh <-chan peer.AddrInfo
	)
	// 在 DHT 包里实现 ttl 验证
	aCh, err = service.routingDiscovery.FindPeers(ctx, ns, discoveryopt.Limit(limit))
	if err != nil {
		return nil, err
	}
	err = errors.New("notfound")
	for a := range aCh {
		if err != nil {
			err = nil
		}
		ret = append(ret, a.ID.Pretty())
	}
	return ret, err
}

func (service *Service) SendMsg(to, protocolID string, msg []byte) (peer.ID, network.Stream, int, error) {
	if service.asc.has(protocolID) {
		return "", nil, 0, fmt.Errorf("This method not support ReuseStream channel (%s), ", protocolID)
	}
	return service.sendMsg(to, protocolID, msg, notimeout)
}

func (service *Service) sendMsg(to, protocolID string, msg []byte, timeout time.Time) (peerid peer.ID, s network.Stream, total int, err error) {
	peerid, err = peer.Decode(to)
	defer func() {
		service.msgc.LogSentMessage(1)
		service.msgc.LogSentMessageStream(1, protocol.ID(protocolID), peerid)
	}()

	if service.asc.has(protocolID) {
		ok, expire := false, false
		if s, ok, expire = service.asc.get(to, protocolID); ok {
			req := NewRawData(nil, msg)
			_total, err2 := ToWriter(s, req)
			if err2 != nil {
				err = err2
				log.Errorf("alibp2p-service::sendMsg-reuse-stream-error-1 to=%s@%s msgid=%d msgsize=%d err=%v", protocolID, to, req.ID(), req.Len(), err2)
				service.asc.del2(to, protocolID, "")
			} else {
				total = int(_total)
				log.Debugf("alibp2p-service::sendMsg-reuse-stream-1 to=%s@%s msgid=%d msgsize=%d", protocolID, to, req.ID(), req.Len())
			}
			return
		} else if expire {
			log.Info("alibp2p-service::sendMsg-gc-expire-stream", "id", to, "pid", protocolID)
			service.asc.del(s)
		}
	}

	if err != nil {
		var (
			ipfsaddr ma.Multiaddr
			pid      string
		)
		ipfsaddr, err = ma.NewMultiaddr(to)
		if err != nil {
			log.Error("alibp2p-service::sendMsg-IDB58Decode-error", "err", err.Error(), "id", to, "pid", protocolID)
			return peerid, nil, 0, err
		}

		pid, err = ipfsaddr.ValueForProtocol(ma.P_IPFS)
		if err != nil {
			log.Error("alibp2p-service::sendMsg-ValueForProtocol-error", "err", err.Error(), "id", to, "pid", protocolID)
			return peerid, nil, 0, err
		}
		peerid, err = peer.Decode(pid)
		if err != nil {
			log.Error("alibp2p-service::sendMsg-IDB58Decode-error", "err", err.Error(), "id", to, "pid", protocolID)
			return peerid, nil, 0, err
		}
		targetPeerAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", peer.IDB58Encode(peerid)))
		targetAddr := ipfsaddr.Decapsulate(targetPeerAddr)
		raddr := []ma.Multiaddr{targetAddr}
		if service.cfg.Relay {
			relayAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/p2p-circuit/ipfs/%s", peer.IDB58Encode(peerid)))
			raddr = append(raddr, relayAddr)
		}
		// TODO
		if raddr == nil || len(raddr) == 0 {
			err = errors.New("no good addrs")
			log.Error("alibp2p-service::sendMsg-addrs-error-1", "err", err.Error(), "id", to, "pid", protocolID)
			return
		}
		service.host.Peerstore().AddAddrs(peerid, raddr, peerstore.TempAddrTTL)
	}

	log.Infof("alibp2p-service::sendMsg-NewStream-start::sendMsg-setDeadline id=%s", to)
	s, err = service.host.NewStream(context.Background(), peerid, protocol.ID(protocolID))
	if err != nil {
		addrs := service.host.Peerstore().Addrs(peerid)
		_addrs := make([]string, 0)
		for _, addr := range addrs {
			_addrs = append(_addrs, addr.String())
		}
		log.Errorf("alibp2p-service::sendMsg-NewStream-error err=%v , id=%s , pid=%s , addr=%v", err.Error(), to, protocolID, _addrs)
		//panic(err)
		return peerid, nil, 0, err
	}

	if timeout != notimeout {
		s.SetWriteDeadline(timeout)
		defer s.SetWriteDeadline(notimeout)
	}
	log.Infof("alibp2p-service::sendMsg-NewStream-end::sendMsg-setDeadline id=%s , timeout=%v", to, timeout)

	if service.asc.has(protocolID) {
		var _total int64
		req := NewRawData(nil, msg)
		_total, err = ToWriter(s, req)
		if err != nil {
			log.Errorf("alibp2p-service::sendMsg-reuse-stream-error-2 to=%s@%s msgid=%d msgsize=%d err=%v", protocolID, to, req.ID(), req.Len(), err)
			return
		} else {
			total = int(_total)
			log.Debugf("alibp2p-service::sendMsg-reuse-stream-2 to=%s@%s msgid=%d msgsize=%d", protocolID, to, req.ID(), req.Len())
		}
		service.asc.put(s)
	} else {
		total, err = s.Write(msg)
		if err != nil {
			log.Errorf("alibp2p-service::sendMsg-reuse-stream-error-3 : err=%v , id=%s , pid=%s", err, to, protocolID)
		}
	}

	return
}

func (service *Service) PreConnect(pubkey *ecdsa.PublicKey) error {
	id, err := peer.IDFromPublicKey(ecdsaToPubkey(pubkey))
	if err != nil {
		log.Error("alibp2p-service::PreConnect-error-1", "id", id.Pretty(), "err", err)
		return err
	}
	pi, err := service.findpeer(id.Pretty())
	if err != nil {
		log.Error("alibp2p-service::PreConnect-error-2", "id", id.Pretty(), "err", err)
		return err
	}
	ctx := context.WithValue(service.ctx, "nodelay", "true")
	err = connectFn(ctx, service.host, []peer.AddrInfo{pi})
	if err != nil {
		log.Error("alibp2p-service::PreConnect-error-3", "id", id.Pretty(), "err", err)
		return err
	}
	log.Debug("alibp2p-service::PreConnect-success : protected", "id", id.Pretty())
	service.host.ConnManager().Protect(id, "pre")
	go func(ctx context.Context, id peer.ID) {
		select {
		case <-time.After(peerstore.TempAddrTTL / 4):
			ok := service.host.ConnManager().Unprotect(id, "pre")
			log.Debug("alibp2p-service::PreConnect-expire : unprotect", "id", id.Pretty(), "ok", ok)
		case <-ctx.Done():
		}
	}(ctx, id)
	return nil
}

// Deprecated: Use OnConnectedEvent.
func (service *Service) OnConnected(t ConnType, preMsg PreMsg, callbackFn ConnectEvent) {
	service.notifiee = append(service.notifiee, &network.NotifyBundle{
		ConnectedF: func(i network.Network, conn network.Conn) {
			switch t {
			case CONNT_TYPE_DIRECT:
				if !service.isDirectFn(conn.RemotePeer().Pretty()) {
					return
				}
			case CONN_TYPE_RELAY:
				if service.isDirectFn(conn.RemotePeer().Pretty()) {
					return
				}
			case CONN_TYPE_ALL:
			}
			var (
				in     bool
				pk, _  = id2pubkey(conn.RemotePeer())
				sid    = fmt.Sprintf("session:%s%s", conn.RemoteMultiaddr().String(), conn.LocalMultiaddr().String())
				pubkey = pubkeyToEcdsa(pk)
				preRtn []byte
			)
			switch conn.Stat().Direction {
			case network.DirInbound:
				in = true
			case network.DirOutbound:
				in = false
			}
			func() {
				// 连出去的，并且 preMsg 有值，就给对方发消息
				if !in && preMsg != nil {
					proto, pkg := preMsg()
					log.Infof("alibp2p-service::OnConnected-send-premsg-start id=%s , pid=%s , pkg=%v", conn.RemotePeer().Pretty(), proto, pkg)
					resp, err := service.RequestWithTimeout(conn.RemotePeer().Pretty(), proto, pkg, 20*time.Second)
					log.Infof("alibp2p-service::OnConnected-send-premsg-end id=%s , pid=%s , rsp=%v , err=%v", conn.RemotePeer().Pretty(), proto, resp, err)
					if err == nil {
						preRtn = resp
					} else {
						preRtn = append(make([]byte, 8), []byte(err.Error())...)
					}
				}
				log.Infof("alibp2p-service::OnConnected-callbackFn-start id=%s", conn.RemotePeer().Pretty())
				callbackFn(in, sid, pubkey, preRtn)
				log.Infof("alibp2p-service::OnConnected-callbackFn-end id=%s", conn.RemotePeer().Pretty())
			}()

		},
	})
}

func (service *Service) OnConnectedEvent(t ConnType, callbackFn ConnectEventFn) {
	service.notifiee = append(service.notifiee, &network.NotifyBundle{
		ConnectedF: func(i network.Network, conn network.Conn) {
			switch t {
			case CONNT_TYPE_DIRECT:
				if !service.isDirectFn(conn.RemotePeer().Pretty()) {
					return
				}
			case CONN_TYPE_RELAY:
				if service.isDirectFn(conn.RemotePeer().Pretty()) {
					return
				}
			case CONN_TYPE_ALL:
			}
			var (
				in     bool
				pk, _  = id2pubkey(conn.RemotePeer())
				sid    = fmt.Sprintf("session:%s%s", conn.RemoteMultiaddr().String(), conn.LocalMultiaddr().String())
				pubkey = pubkeyToEcdsa(pk)
			)
			switch conn.Stat().Direction {
			case network.DirInbound:
				in = true
			case network.DirOutbound:
				in = false
			}
			go func() {
				t := 100 * time.Millisecond
				tc := time.NewTimer(t)
				defer func() {
					tc.Stop()
				}()
				for i := 0; i < 20; i++ {
					/* When IDService successed then to active the OnConnectedEvent , max wait 2s;
					ids.Host.Peerstore().Put(p, "ProtocolVersion", pv)
					ids.Host.Peerstore().Put(p, "AgentVersion", av)
					*/
					_, err1 := service.GetPeerMeta(conn.RemotePeer().Pretty(), "ProtocolVersion")
					_, err2 := service.GetPeerMeta(conn.RemotePeer().Pretty(), "AgentVersion")
					if err1 == nil && err2 == nil {
						log.Infof("alibp2p-service::OnConnected-callbackFn-start id=%s", conn.RemotePeer().Pretty())
						callbackFn(in, sid, pubkey)
						log.Infof("alibp2p-service::OnConnected-callbackFn-end id=%s", conn.RemotePeer().Pretty())
						return
					}
					<-tc.C
					tc.Reset(t)
				}
			}()
		},
	})
}

func (service *Service) RequestWithTimeout(to, proto string, pkg []byte, timeout time.Duration) ([]byte, error) {
	if service.asc.has(proto) {
		log.Infof("alibp2p::RequestWithTimeout-lock:try id=%s , protocolID=%s", to, proto)
		if err := service.asc.takelock(to, proto); err != nil {
			log.Errorf("alibp2p::RequestWithTimeout-lock:fail id=%s , protocolID=%s , err=%s", to, proto, err.Error())
			return nil, err
		}
		log.Infof("alibp2p::RequestWithTimeout-lock:success id=%s , protocolID=%s", to, proto)
		defer service.asc.unlock(to, proto)
	}
	var buf []byte
	tot := notimeout
	if timeout > 0 {
		tot = time.Now().Add(timeout)
	}

	_, s, _, err := service.sendMsg(to, proto, pkg, tot)
	if err == nil {
		if tot != notimeout {
			s.SetReadDeadline(time.Now().Add(timeout))
		} else {
			s.SetReadDeadline(time.Now().Add(20 * time.Second))
		}
		defer func() {
			if s != nil {
				s.SetReadDeadline(notimeout)
				if !service.asc.has(proto) {
					helpers.FullClose(s)
				}
			}
		}()
		if service.asc.has(proto) {
			rsp := new(RawData)
			if _, err = FromReader(s, rsp); err != nil {
				service.asc.del2(to, proto, "")
				log.Errorf("alibp2p::RequestWithTimeout-error-1 %s@%s msgid=%d err=%s", proto, to, rsp.ID(), err.Error())
				return nil, err
			}
			if rsp.Err != "" {
				service.asc.del2(to, proto, "")
				log.Errorf("alibp2p::RequestWithTimeout-error-2 %s@%s msgid=%d err=%s", proto, to, rsp.ID(), rsp.Err)
				return nil, errors.New(rsp.Err)
			}
			log.Debugf("alibp2p::RequestWithTimeout-ack %s@%s msgid=%d", proto, to, rsp.ID())
			buf = rsp.Data
		} else {
			if buf, err = ioutil.ReadAll(s); err != nil {
				return nil, err
			}
		}
	}
	return buf, err
}

func (service *Service) Request(to, proto string, pkg []byte) ([]byte, error) {
	return service.RequestWithTimeout(to, proto, pkg, 0)
}

func (service *Service) OnDisconnected(callback DisconnectEvent) {
	service.notifiee = append(service.notifiee, &network.NotifyBundle{
		DisconnectedF: func(i network.Network, conn network.Conn) {
			pk, _ := id2pubkey(conn.RemotePeer())
			for _, c := range i.Conns() {
				c.RemotePeer()
			}
			sid := fmt.Sprintf("session:%s%s", conn.RemoteMultiaddr().String(), conn.LocalMultiaddr().String())
			callback(sid, pubkeyToEcdsa(pk))
		},
	})
}

func (service *Service) Start() {
	startCounter(service)
	for _, notify := range service.notifiee {
		service.host.Network().Notify(notify)
	}
	if service.cfg.Discover {
		service.bootstrap()
	}
	fmt.Println(">>>> alibp2p-service >>>>")
	fmt.Println("logvsn", logvsn)
	for ns, ttl := range service.nsttl {
		os.Setenv("advertise_"+ns, fmt.Sprintf("%d", ttl))
		fmt.Println("advertise:", ns, " , ttl:", ttl)
	}
	fmt.Println("<<<< alibp2p-service <<<<")
}

func (service *Service) Table() map[string][]string {
	r := make(map[string][]string, 0)
	for _, p := range service.host.Peerstore().Peers() {
		a := make([]string, 0)
		pi := service.host.Peerstore().PeerInfo(p)
		for _, addr := range pi.Addrs {
			a = append(a, addr.String())
		}
		r[p.Pretty()] = a
	}
	return r
}

func (service *Service) GetSession(id string) (session string, inbound bool, err error) {
	err = fmt.Errorf("getsession fail : %s not found.", id)
	for _, conn := range service.host.Network().Conns() {
		if strings.Contains(id, conn.RemotePeer().Pretty()) {
			switch conn.Stat().Direction {
			case network.DirInbound:
				inbound = true
			case network.DirOutbound:
				inbound = false
			}
			session = fmt.Sprintf("session:%s%s", conn.RemoteMultiaddr().String(), conn.LocalMultiaddr().String())
			err = nil
		}
	}
	return session, inbound, err
}

func (service *Service) Conns() (direct []string, relay []string) {
	direct, relay = make([]string, 0), make([]string, 0)
	for _, c := range service.host.Network().Conns() {
		remoteaddr, _ := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", c.RemotePeer().Pretty()))
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

func (service *Service) PeersWithDirection() (direct []PeerDirection, relay map[PeerDirection][]PeerDirection, total int) {
	direct, relay, total = make([]PeerDirection, 0), make(map[PeerDirection][]PeerDirection), 0
	rl := make([]PeerDirection, 0)
	for _, c := range service.host.Network().Conns() {
		remoteaddr, _ := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", c.RemotePeer().Pretty()))
		maddr := c.RemoteMultiaddr().Encapsulate(remoteaddr)
		taddr, _ := maddr.MarshalText()
		if strings.Contains(string(taddr), "p2p-circuit") {
			pd := NewPeerDirection(string(taddr), c.Stat().Direction)
			rl = append(rl, pd)
		} else {
			pd := NewPeerDirection(c.RemotePeer().Pretty(), c.Stat().Direction)
			direct = append(direct, pd)
			total = total + 1
		}
	}
	for _, r := range rl {
		arr := strings.Split(r.ID(), "/p2p-circuit")
		f, t := arr[0][6:], arr[1][6:]
		rarr, ok := relay[NewPeerDirection(f, r.Direction())]
		if !ok {
			rarr = make([]PeerDirection, 0)
		}
		rarr = append(rarr, NewPeerDirection(t, r.Direction()))
		relay[NewPeerDirection(f, r.Direction())] = rarr
		total += 1
	}
	return direct, relay, total
}

func (service *Service) Peers() (direct []string, relay map[string][]string, total int) {
	direct, relay, total = make([]string, 0), make(map[string][]string), 0
	dl, rl := service.Conns()
	for _, d := range dl {
		direct = append(direct, strings.Split(d, "/p2p/")[1])
		total += 1
	}
	for _, r := range rl {
		arr := strings.Split(r, "/p2p-circuit")
		f, t := arr[0], arr[1]

		rarr, ok := relay[strings.Split(f, "/p2p/")[1]]
		if !ok {
			rarr = make([]string, 0)
		}
		rarr = append(rarr, strings.Split(t, "/p2p/")[1])
		relay[strings.Split(f, "/p2p/")[1]] = rarr
		total += 1
	}
	return direct, relay, total
}

func (service *Service) PutPeerMeta(id, key string, v interface{}) error {
	p, err := peer.Decode(id)
	if err != nil {
		return err
	}
	return service.host.Peerstore().Put(p, key, v)
}

func (service *Service) GetPeerMeta(id, key string) (interface{}, error) {
	p, err := peer.Decode(id)
	if err != nil {
		return nil, err
	}
	return service.host.Peerstore().Get(p, key)
}

func (service *Service) Addrs(id string) ([]string, error) {
	peerid, err := peer.Decode(id)
	if err != nil {
		return nil, err
	}
	_addrs := service.host.Peerstore().Addrs(peerid)
	addrs := make([]string, 0)
	for _, addr := range _addrs {
		addrs = append(addrs, addr.String())
	}
	return addrs, nil
}
func (service *Service) Findpeer(id string) ([]string, error) {
	pi, err := service.findpeer(id)
	if err != nil {
		return nil, err
	}
	addrs := make([]string, 0)
	for _, addr := range pi.Addrs {
		if a := addr.String(); !strings.Contains(a, "/p2p-circuit") {
			addrs = append(addrs, addr.String())
		}
	}
	return addrs, nil
}

func (service *Service) findpeer(id string) (peer.AddrInfo, error) {
	peerid, err := peer.Decode(id)
	if err != nil {
		return peer.AddrInfo{}, err
	}
	pi, err := service.router.FindPeer(service.ctx, peerid)
	if err != nil {
		return pi, err
	}
	return pi, nil
}

func (service *Service) GetProtocols(id string) ([]string, error) {
	peerid, err := peer.Decode(id)
	if err != nil {
		return nil, err
	}
	return service.host.Peerstore().GetProtocols(peerid)
}

func (service *Service) Put(k string, v []byte) error {
	return service.router.PutValue(service.ctx, fmt.Sprintf("/%s/%s", NamespaceDHT, k), v)
}

func (service *Service) Get(k string) ([]byte, error) {
	return service.router.GetValue(service.ctx, fmt.Sprintf("/%s/%s", NamespaceDHT, k))
}

func (service *Service) BootstrapOnce() error {
	err := connectFn(context.Background(), service.host, service.bootnodes)
	if err != nil {
		log.Debug("alibp2p-service::bootstrap-once-conn-error", "err", err)
	}
	/* TODO : disable
	err = service.router.(*dht.IpfsDHT).BootstrapOnce(service.ctx, dht.DefaultBootstrapConfig)
	if err != nil {
		log.Debug("alibp2p-service::bootstrap-once-query-error", "err", err)
	}*/
	err = service.router.Bootstrap(service.ctx)
	if err != nil {
		log.Debug("alibp2p-service::bootstrap-once-query-error", "err", err)
	}
	for _, p := range service.bootnodes {
		service.host.Peerstore().AddAddrs(p.ID, p.Addrs, peerstore.PermanentAddrTTL)
	}
	return err
}

func (service *Service) bootstrap() error {
	period := uint64(5)
	if service.cfg.BootstrapPeriod > period {
		period = service.cfg.BootstrapPeriod
	}
	log.Infof("alibp2p-service::host-addrs : %v", service.host.Addrs())
	log.Infof("alibp2p-service::host-network-listen : %v", service.host.Network().ListenAddresses())
	log.Infof("alibp2p-service::host-peerinfo : %v", service.host.Peerstore().PeerInfo(service.host.ID()))
	go func() {
		log.Infof("alibp2p-service::loopboot-start : period=%d", period)
		if atomic.CompareAndSwapInt32(&loopboot, 0, 1) {
			defer func() {
				atomic.StoreInt32(&loopboot, 0)
				atomic.StoreInt32(&loopbootstrap, 0)
			}()
			timer := time.NewTimer(time.Second)
			for {
				select {
				case <-service.ctx.Done():
					return
				case <-timer.C:
					//if service.bootnodes != nil && len(service.host.Network().Conns()) < len(service.bootnodes) {
					if service.bootnodes != nil {
						var (
							limit  = 3
							others = service.peersWithoutBootnodes()
							total  = len(others)
						)
						log.Debug("alibp2p-service::bootstrap looping", service.bootnodes)
						log.Debug("alibp2p-service::connectFn-start")
						err := connectFn(context.Background(), service.host, service.bootnodes)
						log.Debug("alibp2p-service::connectFn-end", err)
						if err == nil {
							// TODO : 新版本 可以重复调用 bootstrap
							log.Debug("alibp2p-service::bootstrap success")
							if atomic.CompareAndSwapInt32(&loopbootstrap, 0, 1) {
								log.Info("alibp2p-service::Bootstrap the host")
								err = service.router.Bootstrap(service.ctx)
								if err != nil {
									log.Infof("alibp2p-service::bootstrap-error : %v", err)
								}
							} else {
								log.Info("alibp2p-service::Reconnected and bootstrap the host once")
								service.BootstrapOnce()
							}
						} else if total > 0 {
							if total < limit {
								limit = total
							}
							tasks := randPeers(others, limit)
							err := connectFn(context.Background(), service.host, tasks)
							log.Infof("alibp2p-service::bootstrap fail try to conn others --> err=%v , total=%d , limit=%d , tasks=%v", err, total, limit, tasks)
						}
					}
				}
				timer.Reset(time.Duration(period) * time.Second)
			}
		}
		log.Info("alibp2p-service::loopboot-end")
	}()
	return nil
}

func (service *Service) peersWithoutBootnodes() []peer.AddrInfo {
	var (
		result  = make([]peer.AddrInfo, 0)
		bootmap = make(map[string]interface{})
	)
	for _, b := range service.bootnodes {
		bootmap[b.ID.Pretty()] = struct{}{}
	}

	for _, p := range service.host.Peerstore().Peers() {
		if _, ok := bootmap[p.Pretty()]; ok {
			continue
		}
		if p.Pretty() == service.host.ID().Pretty() {
			continue
		}

		// 同时也要 exclude 掉 dht.client
		if protols, err := service.GetProtocols(p.Pretty()); err == nil {
			for _, p := range protols {
				if _, ok := service.clientProtocols[p]; ok {
					continue
				}
			}
		}

		if pi := service.host.Peerstore().PeerInfo(p); pi.Addrs != nil && len(pi.Addrs) > 0 {
			result = append(result, pi)
		}
	}

	return result
}

func (service *Service) Nodekey() *ecdsa.PrivateKey {
	return service.cfg.PrivKey
}

//Protect(id, tag string)
//Unprotect(id, tag string) bool
func (service *Service) Protect(id, tag string) error {
	p, err := peer.Decode(id)
	if err != nil {
		return err
	}
	service.host.ConnManager().Protect(p, tag)
	return nil
}

func (service *Service) Unprotect(id, tag string) (bool, error) {
	p, err := peer.Decode(id)
	if err != nil {
		return false, err
	}
	return service.host.ConnManager().Unprotect(p, tag), nil
}

func (service *Service) Report(peerids ...string) []byte {
	now := time.Now().Format("2006-01-02 15:04:05")
	fn := func(stat, stat3 metrics.Stats) string {
		tmp := `{"detail":{"bw":{"total-in":"%d","total-out":"%d","rate-in":"%.2f","rate-out":"%.2f"},"msg":{"total-in":"%d","total-out":"%d","avg-in":"%.2f","avg-out":"%.2f"}}}`
		//tmp := `{"detail":{"bw":{"total-in":"%d","total-out":"%d","rate-in":"%.2f","rate-out":"%.2f"},"rw":{"total-in":"%d","total-out":"%d","avg-in":"%.2f","avg-out":"%.2f"},"msg":{"total-in":"%d","total-out":"%d","avg-in":"%.2f","avg-out":"%.2f"}}}`
		jsonStr := fmt.Sprintf(tmp,
			stat.TotalIn, stat.TotalOut, stat.RateIn, stat.RateOut,
			//stat2.TotalIn, stat2.TotalOut, stat2.RateIn, stat2.RateOut,
			stat3.TotalIn, stat3.TotalOut, stat3.RateIn, stat3.RateOut,
		)
		return jsonStr
	}
	if peerids == nil {
		stat := service.bwc.GetBandwidthTotals()
		//stat2 := service.rwc.GetBandwidthTotals()
		stat3 := service.msgc.GetBandwidthTotals()
		s := fn(stat, stat3)
		return []byte(fmt.Sprintf(`{"time":"%s",%s`, now, s[1:]))
	} else {
		rs := ""
		for _, peerid := range peerids {
			id, err := peer.Decode(peerid)
			if err != nil {
				return []byte(err.Error())
			}
			stat := service.bwc.GetBandwidthForPeer(id)
			//stat2 := service.rwc.GetBandwidthForPeer(id)
			stat3 := service.msgc.GetBandwidthForPeer(id)
			s := fn(stat, stat3)
			ps := fmt.Sprintf(`"%s":%s`, peerid, s)
			rs = rs + ps + ","
		}
		rs = rs[:len(rs)-1]
		return []byte(fmt.Sprintf(`{"time":"%s",%s}`, now, rs))
	}
	return nil
}

func (service *Service) RoutingTable() ([]peer.ID, error) {
	dht, ok := service.router.(*dht.IpfsDHT)
	if !ok {
		return nil, errors.New("router type error")
	}
	return dht.RoutingTable().ListPeers(), nil
}

func (service *Service) Pubsub() TopicService {
	return service
}

func (service *Service) Host() host.Host {
	return service.host
}

func (service *Service) Router() routing.Routing {
	return service.router
}

func (service *Service) Publish(_topic string, msg []byte) error {
	service.topicsMutex.Lock()
	defer service.topicsMutex.Unlock()
	topic, ok := service.topics[_topic]
	if !ok {
		return fmt.Errorf("The topic [%s] notfound", _topic)
	}
	return topic.topic.Publish(service.ctx, msg)
}

func (service *Service) Subscribe(_topic string, callbackFn PubsubCallback) error {
	service.topicsMutex.Lock()
	defer service.topicsMutex.Unlock()
	_, ok := service.topics[_topic]
	if ok {
		return fmt.Errorf("The topic [%s] already subscribe.", _topic)
	}
	topic, err := service.ps.Join(_topic)
	if err != nil {
		return err
	}
	sub, err := topic.Subscribe()
	if err != nil {
		return err
	}
	service.topics[_topic] = &pubsubObj{topic, sub}
	go func() {
		for {
			msg, err := sub.Next(context.Background())
			if err != nil {
				return
			}
			callbackFn(msg.GetFrom(), msg.ReceivedFrom, msg.Data)
		}
	}()
	return nil
}

func (service *Service) Unsubscribe(_topic string) error {
	service.topicsMutex.Lock()
	defer service.topicsMutex.Unlock()
	po, ok := service.topics[_topic]
	if !ok {
		return fmt.Errorf("The topic [%s] notfound", _topic)
	}
	po.sub.Cancel()
	return nil
}
