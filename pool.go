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
 * @Time   : 2020/3/6 3:11 下午
 * @Author : liangc
 *************************************************************************/

package alibp2p

import (
	"bytes"
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p-core/helpers"
	"github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/network"
	"io/ioutil"
	"strings"
	"sync"
)

type (
	reuse_conn struct {
		ctx    context.Context
		reader *bytes.Buffer
		writer *bytes.Buffer
		//rCh    chan *RawData
		//wCh chan *RawData
	}
	StreamKey    string
	SessionKey   string
	AStreamCache struct {
		// { aconnkey -> { session -> conn } }
		pool map[StreamKey]map[SessionKey]network.Stream
		reg  map[string]StreamHandler
		lock *sync.RWMutex
		msgc metrics.Reporter
	}
)

var (
	fullClose = func(s network.Stream) {
		if s != nil {
			stream, session := newStreamSessionKey(s)
			log.Debug("AStreamCache=>fullClose", "streamkey", stream, "session", session)
			go helpers.FullClose(s)
		}
	}
	cleanSession = func(sm map[SessionKey]network.Stream) {
		for _, s := range sm {
			fullClose(s)
		}
	}
	newStreamSessionKey = func(s network.Stream) (stream StreamKey, session SessionKey) {
		stream = newStreamKey(s.Conn().RemotePeer().Pretty(), string(s.Protocol()))
		session = SessionKey(
			fmt.Sprintf("session:%s%s",
				s.Conn().RemoteMultiaddr().String(),
				s.Conn().LocalMultiaddr().String()),
		)
		return
	}
)

func (a *reuse_conn) Read(p []byte) (int, error) {
	i, err := a.reader.Read(p)
	return i, err
	/*if i > 0 {
		return i, err
	}
	select {
	case <-a.ctx.Done():
		return 0, streammux.ErrReset
	case req := <-a.rCh:
		a.reader.Write(req.Data)
		i, err = a.reader.Read(p)
		fmt.Println("RRRRR--1", i, err, "::", p, "::", req.Data)
		return i, err
	}*/
}

func (a *reuse_conn) Write(p []byte) (int, error) {
	return a.writer.Write(p)
	/*
		select {
		case <-a.ctx.Done():
			return 0, streammux.ErrReset
		case a.wCh <- &RawData{Data: p}:
			return len(p), nil
		}*/
}

func newStreamKey(to, protoid string) StreamKey { return StreamKey(protoid + "@" + to) }

func (s StreamKey) Id() string      { return strings.Split(string(s), "@")[1] }
func (s StreamKey) Protoid() string { return strings.Split(string(s), "@")[0] }

func NewAStreamCatch(msgc metrics.Reporter) *AStreamCache {
	return &AStreamCache{
		pool: make(map[StreamKey]map[SessionKey]network.Stream),
		lock: new(sync.RWMutex),
		reg:  make(map[string]StreamHandler),
		//reg:  make(map[string]ReuseStreamHandler),
		msgc: msgc,
	}
}

func (p *AStreamCache) Del(s network.Stream) {
	streamkey, sessionkey := newStreamSessionKey(s)
	p.Del2(streamkey.Id(), streamkey.Protoid(), sessionkey)
}

func (p *AStreamCache) Del2(to, protoid string, session SessionKey) {
	p.lock.Lock()
	defer p.lock.Unlock()
	fmt.Println("AStreamCache.del2-input", to, protoid, session)
	/*
		for k, v := range p.pool {
			fmt.Println("=======>", k)
			for _k, _ := range v {
				fmt.Println("--------------->", _k)
			}
		}
	*/

	if protoid == "" {
		// 1: protoid == nil 删除全部包含 to 的 key, 不会很多，遍历即可
		for streamkey, sm := range p.pool {
			if streamkey.Id() == to {
				cleanSession(sm)
				delete(p.pool, streamkey)
				log.Debug("AStreamCache-del2-1", "id", to, "key", streamkey, "asc.len", len(p.pool))
			}
		}
	} else if session == "" {
		// 2: session == nil 删除 streamkey 下所有 session
		k := newStreamKey(to, protoid)
		cleanSession(p.pool[k])
		delete(p.pool, k)
		log.Debug("AStreamCache-del2-2", "id", to, "protoid", protoid, "key", k, "asc.len", len(p.pool))
	} else if sm, ok := p.pool[newStreamKey(to, protoid)]; ok {
		fullClose(sm[session])
		delete(sm, session)
		log.Debug("AStreamCache-del2-3", "id", to, "protoid", protoid, "session", session, "asc.len", len(p.pool))
		k := newStreamKey(to, protoid)
		if len(sm) == 0 {
			delete(p.pool, k)
		} else {
			p.pool[k] = sm
		}
	}
}

func (p *AStreamCache) Get(to, protoid string) (network.Stream, bool) {
	streamKey := newStreamKey(to, protoid)
	p.lock.RLock()
	defer p.lock.RUnlock()
	sm, ok := p.pool[streamKey]
	if !ok {
		return nil, false
	}
	for _, v := range sm {
		log.Debug("AStreamCache-get", "id", to, "protoid", protoid, "asc.len", len(p.pool))
		return v, true
	}
	return nil, false
}

func (p *AStreamCache) Put(s network.Stream, opts ...interface{}) {
	if opts == nil {
		p.lock.Lock()
		defer p.lock.Unlock()
	}
	streamkey, sessionkey := newStreamSessionKey(s)
	sm, ok := p.pool[streamkey]
	if !ok {
		sm = make(map[SessionKey]network.Stream)
	}
	_, ok = sm[sessionkey]
	if ok {
		//fullClose(old)
		return
	}
	sm[sessionkey] = s
	p.pool[streamkey] = sm
	log.Debug("AStreamCache-put", "id", streamkey.Id(), "protoid", streamkey.Protoid(), "session", sessionkey, "asc.len", len(p.pool))
}

func (p *AStreamCache) Has(pid string) bool {
	if p == nil {
		return false
	}
	_, ok := p.reg[pid]
	return ok
}

func (p *AStreamCache) HandleStream(s network.Stream) {
	pid := string(s.Protocol())
	handlerFn, ok := p.reg[pid]
	if !ok {
		panic("reuse stream pid not found")
	}
	defer func() {
		p.Del(s)
	}()

	log.Debugf("AStreamCache-HandleStream : pid=%s , inbound=%v", pid, s.Conn().Stat().Direction == 1)
	var (
		conn        = s.Conn()
		sid         = fmt.Sprintf("session:%s%s", conn.RemoteMultiaddr().String(), conn.LocalMultiaddr().String())
		pk, _       = id2pubkey(s.Conn().RemotePeer())
		ctx, cancel = context.WithCancel(context.Background())
		//wCh         = make(chan *RawData)
		rw = &reuse_conn{
			ctx:    ctx,
			reader: new(bytes.Buffer),
			writer: new(bytes.Buffer),
			//rCh:    make(chan *RawData),
			//wCh: make(chan *RawData),
		}
		wg = new(sync.WaitGroup)
	)
	// TODO How to return error to the handlerFn ?
	// TODO How to return error to the handlerFn ?
	// TODO How to return error to the handlerFn ?
	// TODO How to return error to the handlerFn ?
	wg.Add(1)
	go func() {
		defer func() {
			log.Debug("reuse stream stop reader : ", pid+"@"+s.Conn().RemotePeer().Pretty())
			wg.Done()
		}()
		//counter := 0
		for {
			req := new(RawData)
			c, err := FromReader(s, req)
			log.Debug("Got a new stream from ", pid+"@"+s.Conn().RemotePeer().Pretty())
			if err != nil {
				log.Debug("HandleStream_reader__error__close-1", pid+"@"+s.Conn().RemotePeer().Pretty(), "err", err, "req", req, "t", c)
				cancel()
				return
			} else if _, err = rw.reader.Write(req.Data); err != nil {
				log.Debug("HandleStream_reader__error__close-2", pid+"@"+s.Conn().RemotePeer().Pretty(), "err", err, "req", req, "t", c)
				cancel()
			} else if err := handlerFn(sid, pubkeyToEcdsa(pk), rw); err != nil {
				log.Debug("HandleStream_handlerFn__error__close-1", pid+"@"+s.Conn().RemotePeer().Pretty(), "err", err, "req", req, "t", c)
				cancel()
				return
			} else if ret, err := ioutil.ReadAll(rw.writer); err != nil {
				log.Debug("HandleStream_handlerFn__error__close-2", pid+"@"+s.Conn().RemotePeer().Pretty(), "err", err, "req", req, "t", c)
				cancel()
				return
			} else if ret != nil {
				//select {
				//case <-ctx.Done():
				//case wCh <- NewRawData(ret):
				//}
				//counter = counter + 1
				//fmt.Println(counter, "<-----", len(ret), string(ret))
				if _, err := ToWriter(s, NewRawData(ret)); err != nil {
					log.Debug("HandleStream_writer__error__close-1", pid+"@"+s.Conn().RemotePeer().Pretty(), err)
					cancel()
					return
				}
				p.msgc.LogSentMessage(1)
				p.msgc.LogSentMessageStream(1, s.Protocol(), s.Conn().RemotePeer())
			}
			select {
			//case rw.rCh <- req:
			//	p.msgc.LogRecvMessage(1)
			//	p.msgc.LogRecvMessageStream(1, s.Protocol(), s.Conn().RemotePeer())
			case <-ctx.Done():
				return
			default:
				p.msgc.LogRecvMessage(1)
				p.msgc.LogRecvMessageStream(1, s.Protocol(), s.Conn().RemotePeer())
			}
		}
	}()
	/*
		go func() {
			defer func() {
				log.Debug("reuse stream stop writer : ", pid+"@"+s.Conn().RemotePeer().Pretty())
				wg.Done()
			}()
			for {
				select {
				case <-ctx.Done():
					return
				case rsp := <-wCh:
					_, err := ToWriter(s, rsp)
					if err != nil {
						log.Debug("HandleStream_writer__error__close-1", pid+"@"+s.Conn().RemotePeer().Pretty(), err)
						cancel()
					}
					p.msgc.LogSentMessage(1)
					p.msgc.LogSentMessageStream(1, s.Protocol(), s.Conn().RemotePeer())
				}
			}
		}()
	*/
	wg.Wait()
	log.Debug("Handler end", pid, "inbound", s.Conn().Stat().Direction == network.DirInbound)
}

/*
func (p *AStreamCache) HandleStream(s network.Stream) {
	pid := string(s.Protocol())
	handlerFn, ok := p.reg[pid]
	if !ok {
		panic("reuse stream pid not found")
	}
	defer func() {
		p.Del(s)
	}()

	fmt.Println("AStreamCache-HandleStream", pid, "inbound", s.Conn().Stat().Direction == network.DirInbound)
	var (
		conn           = s.Conn()
		sid            = fmt.Sprintf("session:%s%s", conn.RemoteMultiaddr().String(), conn.LocalMultiaddr().String())
		pk, _          = id2pubkey(s.Conn().RemotePeer())
		recvCh, sendCh = make(chan *RawData), make(chan *RawData)
		ctx, cancel    = context.WithCancel(context.Background())
	)
	go func() {
		defer log.Debug("reuse stream stop reader : ", pid+"@"+s.Conn().RemotePeer().Pretty())
		for {
			req := new(RawData)
			c, err := FromReader(s, req)
			if err != nil {
				log.Debug("HandleStream_reader__error__close-1", pid+"@"+s.Conn().RemotePeer().Pretty(), "err", err, "req", req, "t", c)
				cancel()
			}
			log.Debug("Got a new stream from ", pid+"@"+s.Conn().RemotePeer().Pretty())
			select {
			case recvCh <- req:
				p.msgc.LogRecvMessage(1)
				p.msgc.LogRecvMessageStream(1, s.Protocol(), s.Conn().RemotePeer())
			case <-ctx.Done():
				return
			}
		}
	}()

	fmt.Println("Reader start", pid, "inbound", s.Conn().Stat().Direction == network.DirInbound)
	go func() {
		defer log.Debug("reuse stream stop writer : ", pid+"@"+s.Conn().RemotePeer().Pretty())
		for {
			select {
			case <-ctx.Done():
				return
			case rsp := <-sendCh:
				_, err := ToWriter(s, rsp)
				if err != nil {
					log.Debug("HandleStream_writer__error__close-1", pid+"@"+s.Conn().RemotePeer().Pretty(), err)
					cancel()
				}
				p.msgc.LogSentMessage(1)
				p.msgc.LogSentMessageStream(1, s.Protocol(), s.Conn().RemotePeer())
			}
		}
	}()
	fmt.Println("Writer start", pid, "inbound", s.Conn().Stat().Direction == network.DirInbound)

	if err := handlerFn(ctx, sid, pubkeyToEcdsa(pk),
		func() (*RawData, error) {
			select {
			case <-ctx.Done():
				return nil, io.EOF
			case req := <-recvCh:
				return req, nil
			}
		},
		func(rsp *RawData) error {
			select {
			case <-ctx.Done():
				return io.EOF
			case sendCh <- rsp:
			}
			return nil
		}); err != nil {
		log.Debug(err)
		cancel()
	}
	fmt.Println("Handler end", pid, "inbound", s.Conn().Stat().Direction == network.DirInbound)
}
*/

func (p *AStreamCache) Reg(pid string, handler StreamHandler) {
	if _, ok := p.reg[pid]; ok {
		panic("ReuseStreamHandler Duplicate Registration")
	}
	p.reg[pid] = handler
}
