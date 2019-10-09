package alibp2p

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/libp2p/go-libp2p"
	apnet "github.com/libp2p/go-libp2p-pnet"
	"strings"

	"math/big"
)

type Config struct {
	//ctx context.Context, homedir string, port int, bootnodes []string
	Ctx       context.Context
	Homedir   string
	Port      uint64
	Bootnodes []string
	Discover  bool
	Networkid *big.Int
}

func (cfg Config) ProtectorOpt() (libp2p.Option, error) {
	if cfg.Networkid != nil {
		s := sha256.New()
		s.Write(cfg.Networkid.Bytes())
		k := s.Sum(nil)
		tmp := `/key/swarm/psk/1.0.0/
/base16/
%s`
		key := fmt.Sprintf(tmp, hex.EncodeToString(k))
		r := strings.NewReader(key)
		p, err := apnet.NewProtector(r)
		if err != nil {
			return nil, err
		}
		return libp2p.PrivateNetwork(p), nil
	}
	return nil, errors.New("disable psk")
}

/*
func NewProtector() (ipnet.Protector, error) {
	if NetworkID == "" {
		return nil, errors.New("protector disable.")
	}
	tmp := `/key/swarm/psk/1.0.0/
/base16/
%s`
	key := fmt.Sprintf(tmp, NetworkID)
	r := strings.NewReader(key)
	return pnet.NewProtector(r)
}
*/
