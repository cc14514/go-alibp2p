package alibp2p

import (
	"crypto/sha256"
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
