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
 * @Time   : 2020/5/13 11:15 上午
 * @Author : liangc
 *************************************************************************/

package alibp2p

import (
	"context"
	"encoding/hex"
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
			b, _ := hex.DecodeString(HexSHA1("pdx"))
			cfg.Networkid = new(big.Int).SetBytes(b)
			log.Infof("Networkid = %s", cfg.Networkid.String())
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
	{
		fallback: func(cfg *Config) bool { return cfg.Groupid == "" },
		opt: func(cfg *Config) error {
			cfg.Groupid = "cc14514"
			return nil
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
