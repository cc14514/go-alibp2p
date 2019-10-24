package alibp2p

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	lru "github.com/hashicorp/golang-lru"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/protocol"
	apnet "github.com/libp2p/go-libp2p-pnet"
	"strings"
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

var (
	pubkeyCache, _          = lru.New(10000)
	DefaultProtocols        = []protocol.ID{ProtocolDHT}
	loopboot, loopbootstrap int32
)

func (cfg Config) ProtectorOpt() (libp2p.Option, error) {
	if cfg.Networkid != nil {
		s := sha256.New()
		s.Write(cfg.Networkid.Bytes())
		k := s.Sum(nil)
		key := fmt.Sprintf(PSK_TMP, hex.EncodeToString(k))
		r := strings.NewReader(key)
		p, err := apnet.NewProtector(r)
		if err != nil {
			return nil, err
		}
		return libp2p.PrivateNetwork(p), nil
	}
	return nil, errors.New("disable psk")
}
