/*************************************************************************
 * Copyright (C) 2016-2019 PDX Technologies, Inc. All Rights Reserved.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 * @Time   : 2019/10/29 3:16 下午
 * @Author : liangc
 *************************************************************************/

package alibp2p

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"io"
	"math/big"
	"net"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAsyncRunner_Apply(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	runner := NewAsyncRunner(ctx, 3, 6)
	go func() {
		for i := 0; i < 12; i++ {
			runner.Apply(func(ctx context.Context, args []interface{}) {
				i := args[0].(int)
				fmt.Println(ctx.Value("tn"), "AAAAAAAAAAA", i, runner.Size())
				time.Sleep(1 * time.Second)
			}, i)
		}
	}()

	go func() {
		for i := 0; i < 10; i++ {
			runner.Apply(func(ctx context.Context, args []interface{}) {
				i := args[0].(int)
				fmt.Println("tn", ctx.Value("tn"), "BBBBBBBBBB", i, "pool-size", runner.Size())
			}, i)
		}
		time.Sleep(3 * time.Second)
		for i := 0; i < 5; i++ {
			time.Sleep(1 * time.Second)
			runner.Apply(func(ctx context.Context, args []interface{}) {
				i := args[0].(int)
				fmt.Println("tn", ctx.Value("tn"), "CCCCCCCCC", i, "pool-size", runner.Size())
			}, i)
		}
		time.Sleep(3 * time.Second)
	}()
	runner.WaitClose()
	fmt.Println("ttl", time.Since(now), runner.Size())
}

func TestAtomic(t *testing.T) {
	var i, j, k int32 = 3, 2, 1
	fmt.Println(i, j, k)
	fmt.Println(atomic.CompareAndSwapInt32(&i, i, k))
	fmt.Println(i, j, k)
}

func TestConnectArgs(t *testing.T) {
	url := "/ip4/39.100.34.235/mux/5978:30200/ipfs/16Uiu2HAmU6ccPRbZpHpTiKo1mJMATudcLgHAsAbUYmADp9Wjn6GJ"
	ipfsaddr, err := ma.NewMultiaddr(url)
	if err != nil {
		t.Error(err)
	}
	pid, err := ipfsaddr.ValueForProtocol(ma.P_IPFS)
	if err != nil {
		t.Error(err)
	}
	peerid, err := peer.IDB58Decode(pid)
	if err != nil {
		t.Error(err)
	}
	targetPeerAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", peer.IDB58Encode(peerid)))
	targetAddr := ipfsaddr.Decapsulate(targetPeerAddr)

	t.Log(peerid)
	t.Log(targetPeerAddr)
	t.Log(targetAddr)
}

func TestTimeout(t *testing.T) {

	notimeout := time.Time{}
	fmt.Println("aaaaaaaaa", time.Time{} == notimeout)

	addr := "127.0.0.1:12345"
	l, _ := net.Listen("tcp", addr)
	connCh := make(chan net.Conn, 0)
	go func() {
		for {
			conn, _ := l.Accept()
			connCh <- conn
		}
	}()

	go func() {
		for {
			fmt.Println("--> Read : ready")
			conn := <-connCh
			fmt.Println("--> Read : accepted")
			buf := make([]byte, 2)
			for {
				n, err := io.ReadFull(conn, buf)
				fmt.Println("--> Read : success", n, err, string(buf))
			}
		}
	}()

	conn, _ := net.Dial("tcp", addr)
	conn.SetWriteDeadline(time.Now().Add(time.Second * 2))
	n, err := conn.Write([]byte("hi"))
	fmt.Println("<-- Write 0 : done", n, err)
	conn.SetWriteDeadline(time.Time{})
	time.Sleep(3 * time.Second)
	n, err = conn.Write([]byte("hi"))
	fmt.Println("<-- Write 1 : done", n, err)
	time.Sleep(time.Second)
}

type (
	Msg struct {
		Id      string
		Type    int
		Payload []byte
	}
	Payload1 struct {
		Name, Address string
	}
)

func TestEncoder(t *testing.T) {

	p := &Payload1{
		Name:    "Hello",
		Address: "Beijing",
	}
	msg := &Msg{
		Id:      "123",
		Type:    1,
		Payload: MustToBytes(p),
	}
	mb, err := ToBytes(msg)
	t.Log("msg1", err, mb)
	t.Log("p1", p)

	msg2 := new(Msg)
	err = FromBytes(mb, msg2)
	t.Log("msg2", err, msg2)
	p2 := new(Payload1)
	FromBytes(msg2.Payload, p2)
	t.Log("p2", p2)
	var i interface{} = p
	i = "a"
	_i := Payload1{
		Name:    "a",
		Address: "b",
	}
	rtn, err := ToBytes(_i)
	t.Log(err, rtn)
	rtn, err = ToBytes(&_i)
	t.Log(err, rtn)

	_j := Payload1{}
	err = FromBytes(rtn, &_j)
	t.Log(err, _j)

	tp := reflect.TypeOf(i)
	t.Log(tp.Kind(), tp.Kind() == reflect.Ptr)

	now := time.Now()
	for a := 0; a < 100000; a++ {
		MustToBytes(msg)
	}
	fmt.Println("amino", time.Since(now), len(MustToBytes(msg)))
	now = time.Now()
	for a := 0; a < 100000; a++ {
		json.Marshal(msg)
	}
	_jr, _ := json.Marshal(msg)
	fmt.Println("json", time.Since(now), len(_jr))
}

type _rw struct {
	reader *bytes.Buffer
}

func (r *_rw) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

func (r *_rw) Write(p []byte) (n int, err error) {
	panic("implement me")
}

func Test_rw(t *testing.T) {
	rw := &_rw{
		reader: new(bytes.Buffer),
	}
	buf := make([]byte, 128)
	i, err := rw.Read(buf)
	t.Log(i, err, buf)
	i, err = rw.Read(buf)
	t.Log(i, err, buf)

	mm := make(map[string]int64)
	x := mm["foo"]
	t.Log(mm, x)
	mm["foo"] = x + 1
	t.Log(mm)
}

func TestUUID(t *testing.T) {
	m := make(map[string]struct{})
	n := time.Now()
	for i := 0; i < 500000; i++ {
		u := uuid.New()
		//k := hex.EncodeToString(u[:])
		m[u.String()] = struct{}{}
	}
	t.Log(time.Since(n), len(m))
}

func TestUUID2(t *testing.T) {
	m := make(map[*big.Int]struct{})
	n := time.Now()
	for i := 0; i < 500000; i++ {
		u := uuid.New()
		k := new(big.Int).SetBytes(u[:])
		m[k] = struct{}{}
	}
	t.Log(time.Since(n), len(m))
}

func TestUUID3(t *testing.T) {
	m := make(map[string]struct{})
	n := time.Now()
	for i := 0; i < 500000; i++ {
		u := uuid.New()
		k := hex.EncodeToString(u[:])
		m[k] = struct{}{}
	}
	t.Log(time.Since(n), len(m))
}

func TestRawMsg(t *testing.T) {
	d := NewRawData(nil, []byte("abc"))
	t.Log(d.Id, d.Len())
	b, _ := ToBytes(d)
	r := new(RawData)
	FromBytes(b, r)
	t.Log(r.Id, r.Len())
}

func TestLock(t *testing.T) {
	s := time.Now()
	m := new(sync.Map)
	for i := 0; i < 1000000; i++ {
		l, _ := m.LoadOrStore(i, new(sync.Mutex))
		l.(*sync.Mutex).Lock()
	}
	fmt.Println("take lock 100w : ", time.Since(s))

	<-time.After(10 * time.Second)
	s = time.Now()
	for i := 0; i < 1000000; i++ {
		//v, _ := m.Load(i)
		//v.(*sync.Mutex).Unlock()
		m.Delete(i)
	}
	fmt.Println("clean lock 100w : ", time.Since(s))

	<-time.After(3600 * time.Second)
}
