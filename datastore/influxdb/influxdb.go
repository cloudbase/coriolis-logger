package influxdb

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	// this is important because of the bug in go mod
	_ "github.com/influxdata/influxdb1-client"
	client "github.com/influxdata/influxdb1-client/v2"
	"github.com/pkg/errors"

	"github.com/gabriel-samfira/coriolis-logger/config"
	"github.com/gabriel-samfira/coriolis-logger/datastore/common"
	"github.com/gabriel-samfira/coriolis-logger/logging"
	"github.com/gabriel-samfira/coriolis-logger/params"
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

func (i *InfluxDBDataStore) ResultReader(p params.QueryParams) common.Reader {
	return &influxDBReader{
		datastore: i,
		params:    p,
	}
}

func (i *InfluxDBDataStore) Query(q client.Query) (*client.ChunkedResponse, error) {
	resp, err := i.con.QueryAsChunk(q)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

type influxDBReader struct {
	datastore *InfluxDBDataStore
	params    params.QueryParams

	result *client.ChunkedResponse
	done   bool
}

func (i *influxDBReader) prepareQuery() (string, error) {
	if i.params.BinaryName == "" {
		return "", fmt.Errorf("missing application name")
	}
	undefinedDate := time.Time{}
	q := fmt.Sprintf(`select timestamp,severity,message from %s`, i.params.BinaryName)
	if !i.params.StartDate.Equal(undefinedDate) || !i.params.EndDate.Equal(undefinedDate) {
		q += ` where `
	}

	dateRanges := []string{}

	if !i.params.StartDate.Equal(undefinedDate) {
		dateRanges = append(
			dateRanges,
			fmt.Sprintf(`timestamp >= %d`, i.params.StartDate.UnixNano()))
	} else if !i.params.EndDate.Equal(undefinedDate) {
		dateRanges = append(
			dateRanges,
			fmt.Sprintf(`timestamp <= %d`, i.params.EndDate.UnixNano()))

	}
	if len(dateRanges) > 0 {
		q += strings.Join(dateRanges, ` and `)
	}
	if i.params.Hostname != "" {
		q += fmt.Sprintf(` and hostname="%s"`, i.params.Hostname)
	}
	if i.params.Severity != 0 {
		q += fmt.Sprintf(` and severity=%d`, i.params.Severity)
	}
	return q, nil
}

var _ common.Reader = (*influxDBReader)(nil)

func (i *influxDBReader) ReadNext() ([]byte, error) {
	if i.result == nil {
		query, err := i.prepareQuery()
		if err != nil {
			return nil, errors.Wrap(err, "preparing query")
		}
		influxQ := client.NewQuery(query, i.datastore.cfg.Database, "ns")
		resp, err := i.datastore.con.QueryAsChunk(influxQ)
		if err != nil {
			return nil, errors.Wrap(err, "executing query")
		}
		i.result = resp
	}

	res, err := i.result.NextResponse()
	if err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, errors.Wrap(err, "reading results")
	}
	buf := bytes.NewBuffer([]byte{})
	for _, result := range res.Results {
		for _, serie := range result.Series {
			for _, val := range serie.Values {
				_, err := buf.Write([]byte(val[2].(string)))
				if err != nil {
					return nil, errors.Wrap(err, "reading value")
				}
			}
		}
	}
	contents := buf.Bytes()
	buf.Reset()
	return contents, nil
}
