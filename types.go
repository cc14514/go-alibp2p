package alibp2p

import (
	"context"
	"crypto/ecdsa"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	"math/big"
	"sync"
)

type (
	Config struct {
		Ctx                   context.Context
		Homedir               string
		Port, ConnLow, ConnHi uint64
		Bootnodes             []string
		Discover              bool
		Networkid, MuxPort    *big.Int

		PrivKey *ecdsa.PrivateKey
	}

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

	asyncFn struct {
		fn   func(context.Context, []interface{})
		args []interface{}
	}
	AsyncRunner struct {
		wg                *sync.WaitGroup
		ctx               context.Context
		counter, min, max int32
		fnCh              chan *asyncFn
		close             bool
	}
)
