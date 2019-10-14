package influxdb

import (
	"time"

	"github.com/gabriel-samfira/coriolis-logger/config"
	"github.com/gabriel-samfira/coriolis-logger/datastore"
)

func NewInfluxDBDatastore(cfg *config.InfluxDB) (datastore.DataStore, error) {
	return &InfluxDBDataStore{
		cfg: cfg,
	}, nil
}

var _ datastore.DataStore = (*InfluxDBDataStore)(nil)

type InfluxDBDataStore struct {
	cfg *config.InfluxDB
}

func (i *InfluxDBDataStore) Write(logParts map[string]interface{}) error {
	return nil
}

func (i *InfluxDBDataStore) Rotate(olderThan time.Duration) error {
	return nil
}
