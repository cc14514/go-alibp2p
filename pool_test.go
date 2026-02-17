// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alibp2p

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestFoobar(t *testing.T) {
	t.Log("check compile")
}

func TChain(c interface{}, v interface{}) bool {
	cv := reflect.ValueOf(c)
	fmt.Println(cv.IsNil(), cv.IsValid(), cv.IsZero())
	defer fmt.Println(cv.IsNil(), cv.IsValid(), cv.IsZero())
	vv := reflect.ValueOf(v)

	ok := cv.TrySend(vv)
	fmt.Println("cv", ok)
	fmt.Println("cv close")
	return ok
}

func TestTChan(t *testing.T) {
	c1 := make(chan int)
	go func() {
		n := <-c1
		fmt.Println("recv:", n)
	}()
	<-time.After(1 * time.Second)
	err := TChain(c1, 10)
	fmt.Println(err)

	<-time.After(1 * time.Second)
}
