// Copyright 2019 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package datastore

import (
	"context"
	"fmt"

	"coriolis-logger/config"
	"coriolis-logger/datastore/common"
	"coriolis-logger/datastore/influxdb"
	"github.com/pkg/errors"
)

func GetDatastore(ctx context.Context, cfg config.Syslog) (common.DataStore, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(err, "validating syslog config")
	}
	switch cfg.DataStore {
	case config.InfluxDBDatastore:
		// Validation should already be done by the config package, but
		// it pays to be paranoid sometimes
		if cfg.InfluxDB == nil {
			return nil, fmt.Errorf("invalid influxdb datastore config")
		}
		return influxdb.NewInfluxDBDatastore(ctx, cfg.InfluxDB)
	default:
		return nil, fmt.Errorf("invalid datastore type")
	}
}
