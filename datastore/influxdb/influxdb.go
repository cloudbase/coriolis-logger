package influxdb

import (
	"sync"
	"time"

	// this is important because of the bug in go mod
	_ "github.com/influxdata/influxdb1-client"
	client "github.com/influxdata/influxdb1-client/v2"
	"github.com/pkg/errors"

	"github.com/gabriel-samfira/coriolis-logger/config"
	"github.com/gabriel-samfira/coriolis-logger/datastore/common"
	"github.com/gabriel-samfira/coriolis-logger/logging"
)

func NewInfluxDBDatastore(cfg *config.InfluxDB) (common.DataStore, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(err, "validating influx config")
	}

	store := &InfluxDBDataStore{
		cfg: cfg,
	}

	if err := store.connect(); err != nil {
		return nil, errors.Wrap(err, "connecting to influxdb")
	}
	return store, nil
}

var _ common.DataStore = (*InfluxDBDataStore)(nil)

type InfluxDBDataStore struct {
	cfg *config.InfluxDB
	con client.Client
	mut sync.Mutex
}

func (i *InfluxDBDataStore) connect() error {
	i.mut.Lock()
	defer i.mut.Unlock()
	tlsCfg, err := i.cfg.TLSConfig()
	if err != nil {
		return errors.Wrap(err, "getting TLS config for influx client")
	}
	conf := client.HTTPConfig{
		Addr:      i.cfg.URL.String(),
		Username:  i.cfg.Username,
		Password:  i.cfg.Password,
		TLSConfig: tlsCfg,
	}
	con, err := client.NewHTTPClient(conf)
	if err != nil {
		return errors.Wrap(err, "getting influx connection")
	}
	i.con = con
	return nil
}

func (i *InfluxDBDataStore) Write(logMsg logging.LogMessage) (err error) {
	defer func() {
		// TODO (gsamfira): revisit this. It may be too heavy handed to reconnect
		// on any error
		if err != nil {
			i.con.Close()
			i.connect()
		}
	}()
	tags := map[string]string{
		"hostname": logMsg.Hostname,
		"severity": logMsg.Severity.String(),
		"facility": logMsg.Facility.String(),
	}
	fields := map[string]interface{}{
		"message": logMsg.Message,
	}
	bp, err := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  i.cfg.Database,
		Precision: "ns",
	})
	if err != nil {
		return errors.Wrap(err, "getting influx batch point")
	}
	var tm time.Time = logMsg.Timestamp
	if logMsg.RFC == logging.RFC3164 {
		tm = time.Now()
	}
	pt, err := client.NewPoint(logMsg.BinaryName, tags, fields, tm)
	if err != nil {
		return errors.Wrap(err, "adding new log message point")
	}
	bp.AddPoint(pt)
	if err := i.con.Write(bp); err != nil {
		return errors.Wrap(err, "writing log line to influx")
	}
	return nil
}

func (i *InfluxDBDataStore) Rotate(olderThan time.Time) error {
	return nil
}

func (i *InfluxDBDataStore) Read(start, end time.Time) ([]byte, error) {
	return nil, nil
}
