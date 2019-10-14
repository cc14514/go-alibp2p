package alibp2p

import (
	"crypto/sha256"
	"fmt"
	"github.com/libp2p/go-libp2p-core/peer"
	"math/big"
	"sync/atomic"
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
	i := int32(0)

	b := atomic.CompareAndSwapInt32(&i, 0, 5)
	t.Log(b, i)
}

func TestByte(t *testing.T) {
	t.Log([]byte("ping"))
}

func TestNodeid(t *testing.T) {
	peerid := "16Uiu2HAmFPq2Tt2TRqAttQHmKiQRcKZ8THmtmQsawAHz84WsHjNr"
	idBytes := []byte(peerid)
	t.Log(len(idBytes), idBytes)
	id, _ := peer.IDB58Decode(peerid)
	pubkey, _ := id.ExtractPublicKey()
	ecdsaPub := pubkeyToEcdsa(pubkey)
	b1, _ := pubkey.Bytes()
	b2 := append(ecdsaPub.X.Bytes(), ecdsaPub.Y.Bytes()...)
	t.Log("b1", len(b1), b1)
	t.Log("b2", len(b2), b2)
	h1 := fmt.Sprintf("%x", b2[:])
	t.Log(len(h1),h1)

}
