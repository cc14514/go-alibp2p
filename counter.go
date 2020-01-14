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
 * @Time   : 2020/1/14 4:51 下午
 * @Author : liangc
 *************************************************************************/

package alibp2p

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	SEND mctype = "s"
	READ mctype = "r"
)

var (
	totalSend, totalRead int64
	cm                   = new(sync.Map)
	kFn                  = func(mt mctype, n int64) string {
		return fmt.Sprintf("%s,%d", READ, n)
	}
)

type mctype string

func startCounter(ctx context.Context) {
	go func() {
		for {
			select {
			case ts := <-time.After(time.Second):
				k := ts.Unix()
				cm.Store(kFn(SEND, k), atomic.LoadInt64(&totalSend))
				cm.Store(kFn(READ, k), atomic.LoadInt64(&totalRead))
			case <-ctx.Done():
				return
			}
		}
	}()

	// gc
	go func() {
		for {
			select {
			case ts := <-time.After(30 * time.Second):
				for k := ts.Unix(); k > ts.Unix()-10; k-- {
					tr1 := kFn(READ, k)
					ts1 := kFn(SEND, k)

					tr2 := kFn(READ, k-1)
					ts2 := kFn(SEND, k-1)

					r1, rok1 := cm.Load(tr1)
					s1, sok1 := cm.Load(ts1)

					r2, rok2 := cm.Load(tr2)
					s2, sok2 := cm.Load(ts2)

					r, s := int64(0), int64(0)
					if rok2 && rok1 && r1 != nil && r2 != nil {
						r = r1.(int64) - r2.(int64)
					}
					if sok2 && sok1 && s1 != nil && s2 != nil {
						s = s1.(int64) - s2.(int64)
					}
					fmt.Println("[ alibp2p-msg-counter : 1sec ]", "ts", k, "r", r, "w", s, "rw", r+s)
				}
				tr, _ := cm.Load(kFn(READ, ts.Unix()-1))
				tw, _ := cm.Load(kFn(SEND, ts.Unix()-1))
				fmt.Println("[ alibp2p-msg-total : 30sec ]", "ts", ts.Unix(), "total-r", tr, "total-w", tw)
				cm.Range(func(key, value interface{}) bool {
					k := strings.Split(key.(string), ",")[1]
					i, _ := new(big.Int).SetString(k, 10)
					if i.Int64() < ts.Unix()-30 {
						cm.Delete(key)
					}
					return true
				})
			case <-ctx.Done():
				return
			}
		}
	}()
}

func addCounter(mct mctype) {
	switch mct {
	case SEND:
		atomic.AddInt64(&totalSend, 1)
	case READ:
		atomic.AddInt64(&totalRead, 1)
	}
}
