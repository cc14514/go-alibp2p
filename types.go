package alibp2p

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/binary"
	"errors"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	discovery "github.com/libp2p/go-libp2p-discovery"
	"io"
	"math/big"
	"sync"
	"time"
)

type (
	RawData struct {
		Id   []byte
		Err  string
		Data []byte
	}

	SimplePacketHead []byte

	Config struct {
		Ctx                                    context.Context
		Homedir                                string
		Port, ConnLow, ConnHi, BootstrapPeriod uint64
		Bootnodes                              []string
		Discover, ReuseStream, Relay           bool
		Networkid, MuxPort                     *big.Int

		PrivKey  *ecdsa.PrivateKey
		Loglevel int // 3 INFO, 4 DEBUG, 5 TRACE -> 3-4 NOTICE , 5 DEBUG
	}

	Service struct {
		ctx              context.Context
		homedir          string
		host             host.Host
		router           routing.Routing
		routingDiscovery *discovery.RoutingDiscovery
		bootnodes        []peer.AddrInfo
		cfg              Config
		notifiee         []*network.NotifyBundle
		isDirectFn       func(id string) bool
		bwc, rwc, msgc   metrics.Reporter
		asc              *AStreamCache
	}
	blankValidator struct{}
	ConnType       int
	asyncFn        struct {
		fn   func(context.Context, []interface{})
		args []interface{}
	}
	AsyncRunner struct {
		sync.Mutex
		wg                *sync.WaitGroup
		ctx               context.Context
		counter, min, max int32
		fnCh              chan *asyncFn
		closeCh           chan struct{}
		close             bool
		gc                time.Duration
	}
)

func NewRawData(id *big.Int, data []byte) *RawData {
	if id == nil {
		u := uuid.New()
		id = new(big.Int).SetBytes(u[:])
	}
	return &RawData{Id: id.Bytes(), Data: data}
}

func ReadSimplePacketHead(r io.Reader) (SimplePacketHead, error) {
	head := make([]byte, 6)
	t, err := r.Read(head)
	if t != 6 || err != nil {
		return nil, err
	}
	return head, nil
}

func NewSimplePacketHead(msgType uint16, data []byte) SimplePacketHead {
	var psize = uint32(len(data))
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, &msgType)
	binary.Write(buf, binary.BigEndian, &psize)
	return buf.Bytes()
}

func (header SimplePacketHead) Decode() (msgType uint16, size uint32, err error) {
	if len(header) != 6 {
		err = errors.New("error_header")
		return
	}
	msgTypeR := bytes.NewReader(header[:2])
	err = binary.Read(msgTypeR, binary.BigEndian, &msgType)
	if err != nil {
		return
	}
	sizeR := bytes.NewReader(header[2:])
	err = binary.Read(sizeR, binary.BigEndian, &size)
	return
}

func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }

func (d *RawData) Len() int     { return len(d.Data) }
func (d *RawData) ID() *big.Int { return new(big.Int).SetBytes(d.Id) }
