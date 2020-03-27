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
 * @Time   : 2020/3/27 10:43 上午
 * @Author : liangc
 *************************************************************************/

package alibp2p

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestStreamLock(t *testing.T) {
	id := "hello"
	pid := "foobar"
	asc := NewAStreamCatch(nil)
	asc.reglock.Store(pid, make(chan struct{}))
	wg := new(sync.WaitGroup)
	s := time.Now()
	m, n := 0, 0
	for j := 0; j < 10000; j++ {
		wg.Add(1)
		go func(i int) {
			defer func() {
				wg.Done()
				err := asc.unlock(id, pid)
				if err != nil {
					m += 1
				}
			}()
			err := asc.takelock(id, pid)
			if err != nil {
				n += 1
			}
		}(j)
	}
	wg.Wait()
	asc.cleanlock(id, pid)
	fmt.Println(time.Since(s), m, n)
}
