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
	"encoding/json"
	"fmt"
	"testing"
)

func TestAddCounter(t *testing.T) {
	s := `
{
  "alibp2p-counter": {
    "bw": {
      "total-in": "643197",
      "total-out": "5696915",
      "rate-in": "20.28",
      "rate-out": "596.35"
    },
    "rw": {
      "total-in": "100219",
      "total-out": "20147",
      "avg-in": "3.17",
      "avg-out": "0.63"
    },
    "msg": {
      "total-in": "9",
      "total-out": "20084",
      "avg-in": "0.00",
      "avg-out": "0.63"
    }
  }
}`
	m := make(map[string]interface{})
	err := json.Unmarshal([]byte(s), &m)
	t.Log(err, m)
	b, err := json.Marshal(m)
	t.Log(err, string(b)[1:])
	h := fmt.Sprintf(`"%s"`, "helloworld")
	t.Log(h)

	now := "now"
	ss := `{"a":123"}`
	bb := []byte(fmt.Sprintf("{\"time\":\"%s\",%s", now, ss[1:]))
	cc := []byte(fmt.Sprintf(`{"time":"%s",{"details":%s}}`, now, `["a","b"]`))
	t.Log(bb)
	t.Log(string(bb))
	t.Log(cc)
	t.Log(string(cc))
}

func TestPool(t *testing.T) {
	var asc AStreamCache
	t.Log(asc.has("hello"))
}
