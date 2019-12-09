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
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"io"
	"net"
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
