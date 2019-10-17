package datastore

import (
	"fmt"

	"github.com/gabriel-samfira/coriolis-logger/config"
	"github.com/gabriel-samfira/coriolis-logger/datastore/common"
	"github.com/gabriel-samfira/coriolis-logger/datastore/influxdb"
	"github.com/pkg/errors"
)

func GetDatastore(cfg config.Syslog) (common.DataStore, error) {
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
		return influxdb.NewInfluxDBDatastore(cfg.InfluxDB)
	default:
		return nil, fmt.Errorf("invalid datastore type")
	}
}
