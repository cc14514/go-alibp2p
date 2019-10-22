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
 * @Time   : 2019/10/21 5:15 下午
 * @Author : liangc
 *************************************************************************/

package netmux

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/transport"
	"github.com/libp2p/go-tcp-transport"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

const P_MUX = 390

var MuxProtocol = ma.Protocol{
	Name:       "mux",
	Code:       P_MUX,
	VCode:      ma.CodeToVarint(P_MUX),
	Size:       32,
	Path:       false,
	Transcoder: new(MuxTranscoder),
}

func init() { ma.AddProtocol(MuxProtocol) }

type (
	MuxTranscoder struct{}
	MuxListener   struct {
		//tl transport.Listener
		maddr ma.Multiaddr
		manet.Listener
	}
	MuxTransport struct {
		tpt *tcp.TcpTransport
	}
)

func (m *MuxListener) Multiaddr() ma.Multiaddr {
	//maddr := m.Listener.Multiaddr()
	//fmt.Println("MMMMMMMMM", maddr, m.maddr)
	//fmt.Println("MMMMMMMMM", maddr, m.maddr)
	return m.maddr
}

func (m *MuxListener) Accept() (manet.Conn, error) {
	return m.Listener.Accept()
}

func NewMuxTransport(tpt *tcp.TcpTransport) *MuxTransport {
	mt := new(MuxTransport)
	mt.tpt = tpt
	return mt
}

func (m MuxTranscoder) StringToBytes(s string) ([]byte, error) {
	ports := strings.Split(s, ":")
	p1, p2 := ports[0], ports[1]
	b1, err := ma.TranscoderPort.StringToBytes(p1)
	if err != nil {
		return nil, err
	}
	b2, err := ma.TranscoderPort.StringToBytes(p2)
	if err != nil {
		return nil, err
	}
	r := append(b1, b2...)
	fmt.Println("StringToBytes : ", s, r)
	return r, nil
}

func (m MuxTranscoder) BytesToString(b []byte) (string, error) {
	s1, err := ma.TranscoderPort.BytesToString(b[:2])
	if err != nil {
		return "", err
	}
	s2, err := ma.TranscoderPort.BytesToString(b[2:])
	if err != nil {
		return "", err
	}
	r := fmt.Sprintf("%s:%s", s1, s2)
	fmt.Println("BytesToString : ", b, r)
	return r, nil
}

func (m MuxTranscoder) ValidateBytes(b []byte) error {
	if len(b) != 4 {
		return errors.New("mux protocol format error")
	}
	return nil
}

func parseMuxargs(raddr ma.Multiaddr) (ip string, fp, tp int, err error) {
	_, ip, err = manet.DialArgs(raddr)
	if err != nil {
		return
	}
	var (
		fport, tport string
		muxAddr      ma.Multiaddr
		addrs        = ma.Split(raddr)
	)
	for _, maddr := range addrs {
		if maddr.Protocols()[0].Code == MuxProtocol.Code {
			muxAddr = maddr
			break
		}
	}
	fport, err = ma.TranscoderPort.BytesToString(muxAddr.Bytes()[2:4])
	if err != nil {
		return
	}
	tport, err = ma.TranscoderPort.BytesToString(muxAddr.Bytes()[4:6])
	if err != nil {
		return
	}

	fp, err = strconv.Atoi(fport)
	if err != nil {
		return
	}
	tp, err = strconv.Atoi(tport)
	if err != nil {
		return
	}
	return
}

func dialMux(ip string, fport, tport int) (conn net.Conn, err error) {
	var (
		t      int
		dialer = &net.Dialer{Timeout: 15 * time.Second}
		addr   = &net.TCPAddr{IP: net.ParseIP(ip), Port: fport}
		req1   = fmt.Sprintf("CONNECT conn://localhost:%d HTTP/1.1\r\nHost: localhost:%d\r\n\r\n", tport, tport)
		buff   = make([]byte, 2048)
	)
	conn, err = dialer.Dial("tcp", addr.String())
	if err != nil {
		log.Println("dialMux-error-1", "err", err, "ip", ip, "fport", fport)
		return
	}
	_, err = conn.Write([]byte(req1))
	t, err = conn.Read(buff)
	if err != nil {
		log.Println("dialMux-error-2", "err", err, "ip", ip, "fport", fport)
		return
	}
	fmt.Println("-- mux [v1] -->", string(buff[:t]))
	if !bytes.Contains(buff[:t], []byte("HTTP/1.1 200")) {
		log.Println("dialMux-error-3", "err", err, "ip", ip, "fport", fport)
		return
	}
	return
}

func (m MuxTransport) Dial(ctx context.Context, raddr ma.Multiaddr, p peer.ID) (transport.CapableConn, error) {
	ip, fport, tport, err := parseMuxargs(raddr)
	if err != nil {
		return nil, err
	}
	fmt.Println(ip, fport, tport)
	c, err := dialMux(ip, fport, tport)
	if err != nil {
		return nil, err
	}
	conn, err := manet.WrapNetConn(c)
	if err != nil {
		return nil, err
	}
	return m.tpt.Upgrader.UpgradeOutbound(ctx, m.tpt, conn, p)
}

func (m MuxTransport) CanDial(addr ma.Multiaddr) bool {
	_, err := addr.ValueForProtocol(P_MUX)
	return err == nil
}

func (m MuxTransport) Listen(laddr ma.Multiaddr) (transport.Listener, error) {
	list, err := m.tpt.GetListen()
	if err != nil {
		return nil, err
	}
	ml := &MuxListener{Listener: list}
	ml.maddr = laddr
	return m.tpt.Upgrader.UpgradeListener(m, ml), nil
}

func (m MuxTransport) Protocols() []int {
	return []int{MuxProtocol.Code}
}

func (m MuxTransport) Proxy() bool {
	return false
}
