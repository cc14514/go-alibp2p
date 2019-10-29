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
		for i := 0; i < 20; i++ {
			time.Sleep(2 * time.Second)
			runner.Apply(func(ctx context.Context, args []interface{}) {
				i := args[0].(int)
				fmt.Println(ctx.Value("tn"), "BBBBBBBBBB", i, runner.Size())
			}, i)
		}
	}()
	runner.WaitClose()
	fmt.Println("ttl", time.Since(now), runner.Size())
}
