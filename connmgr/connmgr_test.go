package connmgr

import (
	"fmt"
	mh "github.com/multiformats/go-multihash"
	"reflect"
	"sync"
	"testing"
)

func TestFoo(t *testing.T) {
	once := new(sync.Once)
	for i := 0; i < 10; i++ {
		once.Do(func() {
			fmt.Println("bar")
		})
	}
}

func ttt(v interface{}) {
	a := reflect.TypeOf(v)
	a.Bits()
	fmt.Println(a.Kind())
}

func TestMh(t *testing.T) {
	// 16 11106adfb183a4a2c94a2f92dab5ade762a4
	// -1 11146adfb183a4a2c94a2f92dab5ade762a47889a5a1
	h, err := mh.Sum([]byte("helloworld"), mh.SHA1, -1)
	t.Log(err, h.HexString())

	ttt("a")
	ttt([]byte("1"))
}
