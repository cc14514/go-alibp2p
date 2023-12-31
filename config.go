package alibp2p

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	lru "github.com/hashicorp/golang-lru"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/pnet"
	mplex "github.com/libp2p/go-libp2p-mplex"
	yamux "github.com/libp2p/go-libp2p-yamux"
	"io/ioutil"
	"math/big"
	"os"
	"strings"
	"time"
)

const logvsn = "0.0.3-rc4-200813-dev-001"

const (
	NamespaceDHT          = "cc14514"
	defConnLow, defConnHi = 50, 500
	PSK_TMP               = `/key/swarm/psk/1.0.0/
/base16/
%s`
)

type ConnType int

const (
	CONNT_TYPE_DIRECT ConnType = iota
	CONN_TYPE_RELAY
	CONN_TYPE_ALL
)

type Config struct {
	Ctx                                           context.Context
	Homedir                                       string
	Port, ConnLow, ConnHi, BootstrapPeriod        uint64
	Bootnodes, ClientProtocols                    []string
	Discover, Relay, DisableInbound, EnableMetric bool
	Networkid, MuxPort                            *big.Int
	PrivKey                                       *ecdsa.PrivateKey
	Loglevel                                      int   // 3 INFO, 4 DEBUG, 5 TRACE -> 3-4 INFO, 5 DEBUG
	MaxMsgSize                                    int64 // min size 1MB (1024*1024)
}

type Option func(cfg *Config) error

func (cfg *Config) Apply(opts ...Option) error {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(cfg); err != nil {
			return err
		}
	}
	return nil
}

type TopicConfig struct {
	throttle int
	timeout  time.Duration
	inline   bool
	valdator PubsubValdator
}

type TopicOption func(cfg *TopicConfig) error

func (cfg *TopicConfig) Apply(opts ...TopicOption) error {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(cfg); err != nil {
			return err
		}
	}
	return nil
}

var (
	pubkeyCache, _ = lru.New(10000)
	//DefaultProtocols        = []protocol.ID{ProtocolDHT}
	loopboot, loopbootstrap int32
	//defReadTimeout          = 25 * time.Second
	//defWriteTimeout         = 15 * time.Second
	notimeout = time.Time{}
	//def_maxsize  int64 = 512
	def_maxsize int64 = 30 * 1024 * 1024 // TODO just for debug
	//def_maxsize int64 = 13 * 1024 * 1024
	//_maxsize  int64 = 13 * 1024 * 1024
	def_nsttl = time.Duration(86400) // 1hour
)

func (cfg *Config) ProtectorOpt() (libp2p.Option, error) {
	if cfg.Networkid != nil {
		s := sha256.New()
		s.Write(cfg.Networkid.Bytes())
		k := s.Sum(nil)
		key := fmt.Sprintf(PSK_TMP, hex.EncodeToString(k))
		r := strings.NewReader(key)
		psk, err := pnet.DecodeV1PSK(r)
		if err != nil {
			return nil, err
		}
		return libp2p.PrivateNetwork(psk), nil
	}
	return nil, errors.New("disable psk")
}

/*
// DefaultConfig is used to return a default configuration
func DefaultConfig() *Config {
	return &Config{
		AcceptBacklog:          256,
		EnableKeepAlive:        true,
		KeepAliveInterval:      30 * time.Second,
		ConnectionWriteTimeout: 10 * time.Second,
		MaxStreamWindowSize:    initialStreamWindow,
		LogOutput:              os.Stderr,
		ReadBufSize:            4096,
		MaxMessageSize:         64 * 1024, // Means 64KiB/10s = 52kbps minimum speed.
		WriteCoalesceDelay:     100 * time.Microsecond,
	}
}
*/
func (cfg Config) MuxTransportOption(loglevel int) libp2p.Option {
	ymxtpt := &yamux.Transport{
		AcceptBacklog:          256,
		EnableKeepAlive:        true,
		KeepAliveInterval:      45 * time.Second,
		ConnectionWriteTimeout: 45 * time.Second,
		MaxStreamWindowSize:    uint32(256 * 1024),
		//LogOutput:              ioutil.Discard,
		ReadBufSize:        4096,
		MaxMessageSize:     128 * 1024, // Means 128KiB/10s
		WriteCoalesceDelay: 100 * time.Microsecond,
	}

	switch loglevel {
	case 3, 4, 5:
		ymxtpt.LogOutput = os.Stderr
	default:
		ymxtpt.LogOutput = ioutil.Discard
	}

	return libp2p.ChainOptions(
		libp2p.Muxer("/yamux/1.0.0", ymxtpt),
		libp2p.Muxer("/mplex/6.7.0", mplex.DefaultTransport),
	)

}
