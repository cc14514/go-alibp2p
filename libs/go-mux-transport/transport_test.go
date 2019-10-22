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
 * @Time   : 2019/10/21 8:05 下午
 * @Author : liangc
 *************************************************************************/

package netmux

import (
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
	"testing"
	"time"
)
var (
	muxAddr = "/ip4/127.0.0.1/mux/5978:10000/ipfs/16Uiu2HAm78he27L4fkw7es8QxJCPzSNLWmwC7ZN9rwbCkK47d1AX"
)

func TestParseMuxargs(t *testing.T) {
	maddr, _ := ma.NewMultiaddr(muxAddr)
	ip, fport, tport, err := parseMuxargs(maddr)
	t.Log(err, ip, fport, tport)
}

func TestDialMux(t *testing.T) {
	maddr, _ := ma.NewMultiaddr(muxAddr)
	ip, fport, tport, _ := parseMuxargs(maddr)
	conn, err := dialMux(ip, fport, tport)
	t.Log(err, conn)
	time.Sleep(30*time.Second)
}

func TestS(t *testing.T) {
	i, j, err := ma.ReadVarintCode([]byte("tcp"))
	t.Log(err, i, j)
	i, j, err = ma.ReadVarintCode([]byte("mux"))
	t.Log(err, i, j)
}

func TestMaddr(t *testing.T) {
	maddr1, err := ma.NewMultiaddr("/ip4/127.0.0.1/mux/10000:20000/ipfs/16Uiu2HAm78he27L4fkw7es8QxJCPzSNLWmwC7ZN9rwbCkK47d1AX")
	t.Log(err, maddr1)
	maddr2, err := ma.NewMultiaddr("/ip4/39.100.39.60/tcp/10000/ipfs/16Uiu2HAm78he27L4fkw7es8QxJCPzSNLWmwC7ZN9rwbCkK47d1AX")
	t.Log(err, maddr2)
	maddr3, err := ma.NewMultiaddr("/ipfs/16Uiu2HAkxPyGRhydQNwxXRZCLqoc9TgqmWP5ypqneVfpRxGu8GXj/p2p-circuit/ipfs/16Uiu2HAkxGWstVk7FTqjZMBbF8j4qZ8jeTr3FxZnMoLWuUxePnwN")
	t.Log(err, maddr3)

	t.Log("1", ma.Split(maddr1), maddr1.Protocols())
	t.Log("2", ma.Split(maddr2), maddr2.Protocols())
	t.Log("3", ma.Split(maddr3), maddr3.Protocols())
	t.Log("----------------------")
	var muxAddr ma.Multiaddr
	addrs := ma.Split(maddr1)
	for _, maddr := range addrs {
		if maddr.Protocols()[0].Code == MuxProtocol.Code {
			muxAddr = maddr
			break
		}
	}
	a, b, _ := manet.DialArgs(maddr1)
	s1, _ := ma.TranscoderPort.BytesToString(muxAddr.Bytes()[2:4])
	s2, _ := ma.TranscoderPort.BytesToString(muxAddr.Bytes()[4:6])

	t.Log(a, b, s1, s2)

}
