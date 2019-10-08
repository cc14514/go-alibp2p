package alibp2p

import (
	"crypto/sha256"
	"github.com/libp2p/go-libp2p-core/peer"
	"math/big"
	"testing"
)

func TestPSK(t *testing.T) {
	n := big.NewInt(100)
	s := sha256.New()
	s.Write(n.Bytes())
	h := s.Sum(nil)
	t.Log(n.Bytes())
	t.Log(len(h), h)
}

func TestID(t *testing.T) {
	peerid, err := peer.IDB58Decode("16Uiu2HAmFPq2Tt2TRqAttQHmKiQRcKZ8THmtmQsawAHz84WsHjNr")
	t.Log(err, peerid)
}
