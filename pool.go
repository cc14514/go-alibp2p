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
		pool    map[StreamKey]map[SessionKey]network.Stream
		reg     map[string]StreamHandler
		reglock map[string]*sync.Mutex
		lock    *sync.RWMutex
		msgc    metrics.Reporter
	}
)

var (
	fullClose = func(s network.Stream) {
		if s != nil {
			stream, session := newStreamSessionKey(s)
			log.Debug("AStreamCache-fn->fullClose", "streamkey", stream, "session", session)
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
	return a.reader.Read(p)
}

func (a *reuse_conn) Write(p []byte) (int, error) {
	return a.writer.Write(p)
}

func newStreamKey(to, protoid string) StreamKey { return StreamKey(protoid + "@" + to) }

func (s StreamKey) Id() string      { return strings.Split(string(s), "@")[1] }
func (s StreamKey) Protoid() string { return strings.Split(string(s), "@")[0] }

func NewAStreamCatch(msgc metrics.Reporter) *AStreamCache {
	return &AStreamCache{
		pool:    make(map[StreamKey]map[SessionKey]network.Stream),
		lock:    new(sync.RWMutex),
		reg:     make(map[string]StreamHandler),
		reglock: make(map[string]*sync.Mutex),
		msgc:    msgc,
	}
}

func (p *AStreamCache) del(s network.Stream) {
	streamkey, sessionkey := newStreamSessionKey(s)
	p.del2(streamkey.Id(), streamkey.Protoid(), sessionkey)
}

func (p *AStreamCache) del2(to, protoid string, session SessionKey) {
	p.lock.Lock()
	defer p.lock.Unlock()
	log.Debug("AStreamCache-del2.input", to, protoid, session)
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

func (p *AStreamCache) get(to, protoid string) (network.Stream, bool) {
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

func (p *AStreamCache) put(s network.Stream, opts ...interface{}) {
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

func (p *AStreamCache) has(pid string) bool {
	if p == nil {
		return false
	}
	_, ok := p.reg[pid]
	return ok
}

func (p *AStreamCache) handleStream(s network.Stream) {
	pid := string(s.Protocol())
	log.Debugf("AStreamCache-HandleStream-start : pid=%s , inbound=%v", pid, s.Conn().Stat().Direction == network.DirInbound)
	defer func() {
		log.Debugf("AStreamCache-HandleStream-end : pid=%s , inbound=%v", pid, s.Conn().Stat().Direction == network.DirInbound)
		p.del(s)
	}()
	var (
		conn        = s.Conn()
		sid         = fmt.Sprintf("session:%s%s", conn.RemoteMultiaddr().String(), conn.LocalMultiaddr().String())
		pk, _       = id2pubkey(s.Conn().RemotePeer())
		ctx, cancel = context.WithCancel(context.Background())
		rw          = &reuse_conn{
			ctx:    ctx,
			reader: new(bytes.Buffer),
			writer: new(bytes.Buffer),
		}
	)
	// TODO How to return error to the handlerFn ?
	for {
		req := new(RawData)
		c, err := FromReader(s, req)
		log.Debug("Got a new stream from ", pid+"@"+s.Conn().RemotePeer().Pretty())
		if err != nil {
			log.Error("HandleStream_reader", pid+"@"+s.Conn().RemotePeer().Pretty(), "size", c, "err", err)
			cancel()
		} else if _, err = rw.reader.Write(req.Data); err != nil {
			log.Error("HandleStream_rw", pid+"@"+s.Conn().RemotePeer().Pretty(), "size", c, "err", err)
			cancel()
		} else if err = p.reg[pid](sid, pubkeyToEcdsa(pk), rw); err != nil {
			log.Error("HandleStream_fn", pid+"@"+s.Conn().RemotePeer().Pretty(), "size", c, "err", err)
			cancel()
		} else if ret, err := ioutil.ReadAll(rw.writer); err != nil {
			log.Error("HandleStream_ret", pid+"@"+s.Conn().RemotePeer().Pretty(), "size", c, "err", err)
			cancel()
		} else if ret != nil {
			if _, err := ToWriter(s, NewRawData(ret)); err != nil {
				log.Error("HandleStream_writer", pid+"@"+s.Conn().RemotePeer().Pretty(), "size", len(ret), err)
				cancel()
			}
			p.msgc.LogSentMessage(1)
			p.msgc.LogSentMessageStream(1, s.Protocol(), s.Conn().RemotePeer())
		}
		select {
		case <-ctx.Done():
			return
		default:
			p.msgc.LogRecvMessage(1)
			p.msgc.LogRecvMessageStream(1, s.Protocol(), s.Conn().RemotePeer())
		}
	}
}

func (p *AStreamCache) regist(pid string, handler StreamHandler) {
	if _, ok := p.reg[pid]; ok {
		panic("ReuseStreamHandler Duplicate Registration")
	}
	p.reg[pid] = handler
	p.reglock[pid] = new(sync.Mutex)
}

func (p *AStreamCache) lockpid(pid string) {
	lock, ok := p.reglock[pid]
	if ok {
		lock.Lock()
	}
}

func (p *AStreamCache) unlockpid(pid string) {
	lock, ok := p.reglock[pid]
	if ok {
		lock.Unlock()
	}
}
