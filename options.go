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
	"crypto/ecdsa"
	"math/big"
)

/*
type Config struct {
	Ctx                                           context.Context
	Homedir                                       string
	Port, ConnLow, ConnHi, BootstrapPeriod        uint64
	Bootnodes, ClientProtocols                    []string
	Discover, Relay, DisableInbound, EnableMetric bool
	Networkid, MuxPort                            *big.Int
	PrivKey                                       *ecdsa.PrivateKey
	Loglevel                                      int   // 3 INFO, 4 DEBUG, 5 TRACE -> 3-4 INFO, 5 DEBUG
	MaxMsgSize                                    int64 // min size 1MB (1024*1024)
}
*/

func MaxMsgSize(size int64) Option {
	return func(cfg *Config) error {
		if size < 1024*1024 {
			size = 1024 * 1024
		}
		cfg.MaxMsgSize = size
		return nil
	}
}

func Loglevel(level int) Option {
	return func(cfg *Config) error {
		cfg.Loglevel = level
		return nil
	}
}

func Identity(priv *ecdsa.PrivateKey) Option {
	return func(cfg *Config) error {
		cfg.PrivKey = priv
		return nil
	}
}

func Network(port, muxport, networkid uint64) Option {
	return func(cfg *Config) error {
		mp := new(big.Int)
		nid := new(big.Int)
		if muxport > 0 {
			mp = mp.SetUint64(muxport)
		}
		if networkid > 0 {
			nid = nid.SetUint64(networkid)
		}
		cfg.Networkid = nid
		cfg.Port = port
		cfg.MuxPort = mp
		return nil
	}
}

func Bootnodes(bootnodes ...string) Option {
	return func(cfg *Config) error {
		cfg.Bootnodes = bootnodes
		return nil
	}
}

func ClientProtocols(protocols ...string) Option {
	return func(cfg *Config) error {
		cfg.ClientProtocols = protocols
		return nil
	}
}

func Context(ctx context.Context) Option {
	return func(cfg *Config) error {
		cfg.Ctx = ctx
		return nil
	}
}

func Homedir(dir string) Option {
	return func(cfg *Config) error {
		cfg.Homedir = dir
		return nil
	}
}

func ConnectManger(bootstrapPeriod, connLow, connHi uint64) Option {
	return func(cfg *Config) error {
		cfg.BootstrapPeriod = bootstrapPeriod
		cfg.ConnLow, cfg.ConnHi = connLow, connHi
		return nil
	}
}

func EnableMetric(EnableMetric bool) Option {
	return func(cfg *Config) error {
		cfg.EnableMetric = EnableMetric
		return nil
	}
}

func Discover(Discover bool) Option {
	return func(cfg *Config) error {
		cfg.Discover = Discover
		return nil
	}
}

func Relay(Relay bool) Option {
	return func(cfg *Config) error {
		cfg.Relay = Relay
		return nil
	}
}

func DisableInbound(DisableInbound bool) Option {
	return func(cfg *Config) error {
		cfg.DisableInbound = DisableInbound
		return nil
	}
}

func WithValidator(v PubsubValdator) TopicOption {
	return func(cfg *TopicConfig) error {
		cfg.valdator = v
		return nil
	}
}
