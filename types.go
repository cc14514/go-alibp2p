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
	lru "github.com/hashicorp/golang-lru"
	pool "github.com/libp2p/go-buffer-pool"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-core/routing"
	ma "github.com/multiformats/go-multiaddr"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"path"
	"sync"
)

const (
	ProtocolDHT           protocol.ID = "/pdx/kad/1.0.0"
	NamespaceDHT                      = "cc14514"
	defConnLow, defConnHi             = 50, 500
	PSK_TMP                           = `/key/swarm/psk/1.0.0/
/base16/
%s`
)

const (
	CONNT_TYPE_DIRECT ConnType = iota
	CONN_TYPE_RELAY
	CONN_TYPE_ALL
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

func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }

var (
	pubkeyCache, _          = lru.New(10000)
	loopboot, loopbootstrap int32
	DefaultProtocols        = []protocol.ID{ProtocolDHT}
	curve                   = btcec.S256()
)

// private funcs
var (
	loadid = func(homedir string) (crypto.PrivKey, error) {
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
	id2pubkey = func(id peer.ID) crypto.PubKey {
		v, ok := pubkeyCache.Get(id)
		if ok {
			return v.(crypto.PubKey)
		}
		k, _ := id.ExtractPublicKey()
		pubkeyCache.Add(id, k)
		return k
	}
	randPeers = func(others []peer.AddrInfo, limit int) []peer.AddrInfo {
		_, randk, _ := crypto.GenerateRSAKeyPair(1024, rand.Reader)
		rnBytes, _ := randk.Bytes()
		n := new(big.Int).Mod(new(big.Int).SetBytes(rnBytes), big.NewInt(int64(len(others)))).Int64()
		others = append(others[n:], others[:n]...)
		others = others[:limit]
		return others
	}
)

// public funcs
var (
	ECDSAPubEncode = func(pk *ecdsa.PublicKey) (string, error) {
		id, err := peer.IDFromPublicKey(ecdsaToPubkey(pk))
		return id.Pretty(), err
	}
	ECDSAPubDecode = func(pk string) (*ecdsa.PublicKey, error) {
		id, err := peer.IDB58Decode(pk)
		if err != nil {
			return nil, err
		}
		pub := id2pubkey(id)
		return pubkeyToEcdsa(pub), nil
	}

	GetBuf = func(size int) []byte {
		return pool.Get(size)
	}
	PutBuf = func(buf []byte) {
		pool.Put(buf)
	}
)

