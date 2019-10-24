package alibp2p

import (
	"context"
	"crypto/ecdsa"
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
	Ctx                   context.Context
	Homedir               string
	Port, ConnLow, ConnHi uint64
	Bootnodes             []string
	Discover              bool
	Networkid, MuxPort    *big.Int

	PrivKey *ecdsa.PrivateKey
}

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
