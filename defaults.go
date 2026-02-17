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
	"context"
	"errors"
	"math/big"
)

var defaults = []struct {
	fallback func(cfg *Config) bool
	opt      Option
}{
	{
		fallback: func(cfg *Config) bool { return cfg.Ctx == nil },
		opt: func(cfg *Config) error {
			cfg.Ctx = context.Background()
			return nil
		},
	},
	{
		fallback: func(cfg *Config) bool { return cfg.Networkid == nil },
		opt: func(cfg *Config) error {
			cfg.Networkid = big.NewInt(111)
			return nil
		},
	},
	{
		fallback: func(cfg *Config) bool { return cfg.BootstrapPeriod == 0 },
		opt: func(cfg *Config) error {
			cfg.BootstrapPeriod = 45
			return nil
		},
	},
	{
		fallback: func(cfg *Config) bool { return cfg.Loglevel == 0 },
		opt: func(cfg *Config) error {
			cfg.Loglevel = 3
			return nil
		},
	},
	{
		fallback: func(cfg *Config) bool { return cfg.ConnLow == 0 },
		opt: func(cfg *Config) error {
			cfg.ConnLow = 50
			return nil
		},
	},
	{
		fallback: func(cfg *Config) bool { return cfg.ConnHi == 0 },
		opt: func(cfg *Config) error {
			cfg.ConnHi = 200
			return nil
		},
	},
	{
		fallback: func(cfg *Config) bool { return cfg.PrivKey == nil && cfg.Homedir == "" },
		opt: func(cfg *Config) error {
			return errors.New("privateKey and homedir both empty")
		},
	},
}

var FallbackDefaults Option = func(cfg *Config) error {
	for _, def := range defaults {
		if !def.fallback(cfg) {
			continue
		}
		if err := cfg.Apply(def.opt); err != nil {
			return err
		}
	}
	return nil
}
