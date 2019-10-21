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

package influxdb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	// this is important because of the bug in go mod
	_ "github.com/influxdata/influxdb1-client"
	client "github.com/influxdata/influxdb1-client/v2"
	"github.com/juju/loggo"
	"github.com/pkg/errors"

	"github.com/gabriel-samfira/coriolis-logger/config"
	"github.com/gabriel-samfira/coriolis-logger/datastore/common"
	"github.com/gabriel-samfira/coriolis-logger/logging"
	"github.com/gabriel-samfira/coriolis-logger/params"
)

var log = loggo.GetLogger("coriolis.logger.datastore.influxdb")

func NewInfluxDBDatastore(ctx context.Context, cfg *config.InfluxDB) (common.DataStore, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(err, "validating influx config")
	}

	store := &InfluxDBDataStore{
		cfg:    cfg,
		points: []*client.Point{},
		ctx:    ctx,
		closed: make(chan struct{}),
		quit:   make(chan struct{}),
	}

	if err := store.connect(); err != nil {
		return nil, errors.Wrap(err, "connecting to influxdb")
	}
	return store, nil
}

var _ common.DataStore = (*InfluxDBDataStore)(nil)

type InfluxDBDataStore struct {
	cfg    *config.InfluxDB
	con    client.Client
	mut    sync.Mutex
	points []*client.Point
	ctx    context.Context
	closed chan struct{}
	quit   chan struct{}
}

func (i *InfluxDBDataStore) doWork() {
	var interval int
	if i.cfg.WriteInterval == 0 {
		interval = 1
	} else {
		interval = i.cfg.WriteInterval
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	rotationTicker := time.NewTicker(1 * time.Hour)
	defer func() {
		ticker.Stop()
		rotationTicker.Stop()
		close(i.closed)
	}()
	for {
		select {
		case <-i.ctx.Done():
			return
		case <-ticker.C:
			if err := i.flush(); err != nil {
				log.Errorf("failed to flush logs to backend: %v", err)
			}
		case <-rotationTicker.C:
			retentionPeriod := i.cfg.GetLogRetention()
			log.Infof("deleting logs older than %d days", retentionPeriod)
			now := time.Now()
			day := 24 * time.Hour
			olderThan := now.Add(time.Duration(-retentionPeriod) * day)
			if err := i.Rotate(olderThan); err != nil {
				log.Errorf("failed to rotate logs: %v", err)
			}
		case <-i.quit:
			return
		}
	}
}

func (i *InfluxDBDataStore) Start() error {
	go i.doWork()
	return nil
}

func (i *InfluxDBDataStore) Stop() error {
	close(i.quit)
	i.Wait()
	return nil
}

func (i *InfluxDBDataStore) Wait() {
	<-i.closed
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

func (i *InfluxDBDataStore) flush() error {
	i.mut.Lock()
	defer i.mut.Unlock()
	bp, err := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  i.cfg.Database,
		Precision: "ns",
	})
	if err != nil {
		return errors.Wrap(err, "getting influx batch point")
	}
	if i.points != nil && len(i.points) > 0 {
		for _, val := range i.points {
			bp.AddPoint(val)
		}
		if err := i.con.Write(bp); err != nil {
			return errors.Wrap(err, "writing log line to influx")
		}
		i.points = []*client.Point{}
	}
	return nil
}

func (i *InfluxDBDataStore) Write(logMsg logging.LogMessage) (err error) {
	if len(i.points) >= 20000 {
		if err := i.flush(); err != nil {
			return errors.Wrap(err, "flushing logs")
		}
	}

	i.mut.Lock()
	defer i.mut.Unlock()
	tags := map[string]string{
		"hostname": logMsg.Hostname,
		"severity": logMsg.Severity.String(),
		"facility": logMsg.Facility.String(),
	}
	fields := map[string]interface{}{
		"message": logMsg.Message,
	}

	var tm time.Time = logMsg.Timestamp
	if logMsg.RFC == logging.RFC3164 {
		tm = time.Now()
	}
	pt, err := client.NewPoint(logMsg.AppName, tags, fields, tm)
	if err != nil {
		return errors.Wrap(err, "adding new log message point")
	}
	i.points = append(i.points, pt)

	return nil
}

func (i *InfluxDBDataStore) Rotate(olderThan time.Time) error {
	logList, err := i.List()
	if err != nil {
		return errors.Wrap(err, "listing logs")
	}
	for _, val := range logList {
		for _, logName := range val {
			q := fmt.Sprintf(`delete from "%s" where time < %d`, logName, olderThan.UnixNano())
			influxQ := client.NewQuery(q, i.cfg.Database, "ns")
			resp, err := i.con.Query(influxQ)
			if err != nil {
				return errors.Wrap(err, "executing query")
			}
			if resp.Err != "" {
				return fmt.Errorf("error executing query: %s", resp.Err)
			}
		}
	}
	return nil
}

func (i *InfluxDBDataStore) ResultReader(p params.QueryParams) common.Reader {
	return &influxDBReader{
		datastore: i,
		params:    p,
	}
}

func (i *InfluxDBDataStore) List() ([]map[string]string, error) {
	query := client.NewQuery("SHOW MEASUREMENTS", i.cfg.Database, "ns")
	resp, err := i.con.QueryAsChunk(query)
	if err != nil {
		return nil, errors.Wrap(err, "listing logs")
	}
	ret := []map[string]string{}
	for {
		r, err := resp.NextResponse()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, errors.Wrap(err, "fetching response")
		}
		if r.Err != "" {
			return nil, fmt.Errorf("error executing query: %s", r.Err)
		}
		for _, result := range r.Results {
			for _, serie := range result.Series {
				for _, val := range serie.Values {
					if len(val) == 0 {
						continue
					}
					ret = append(ret, map[string]string{"log_name": val[0].(string)})
				}
			}
		}
	}
	return ret, nil
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
	if i.params.AppName == "" {
		return "", fmt.Errorf("missing application name")
	}
	undefinedDate := time.Time{}
	q := fmt.Sprintf(`select time,severity,message from "%s"`, i.params.AppName)
	if !i.params.StartDate.Equal(undefinedDate) || !i.params.EndDate.Equal(undefinedDate) || i.params.Hostname != "" {
		q += ` where `
	}

	options := []string{}

	if !i.params.StartDate.Equal(undefinedDate) {
		options = append(
			options,
			fmt.Sprintf(`time >= %d`, i.params.StartDate.UnixNano()))
	}

	if !i.params.EndDate.Equal(undefinedDate) {
		options = append(
			options,
			fmt.Sprintf(`time <= %d`, i.params.EndDate.UnixNano()))

	}
	if i.params.Hostname != "" {
		options = append(options, fmt.Sprintf(`hostname='%s'`, i.params.Hostname))
	}

	if len(options) > 0 {
		q += strings.Join(options, ` and `)
	}

	return q, nil
}

var _ common.Reader = (*influxDBReader)(nil)

func (i *influxDBReader) ReadNext() ([]byte, error) {
	if i.result == nil {
		i.datastore.flush()
		query, err := i.prepareQuery()
		if err != nil {
			return nil, errors.Wrap(err, "preparing query")
		}
		influxQ := client.NewQuery(query, i.datastore.cfg.Database, "ns")
		influxQ.ChunkSize = 20000
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
	newline := []byte("\n")
	buf := bytes.NewBuffer([]byte{})
	for _, result := range res.Results {
		for _, serie := range result.Series {
			for _, val := range serie.Values {
				line := []byte(val[2].(string))
				if len(line) > 0 && line[len(line)-1] != newline[0] {
					line = append(line, []byte("\n")...)
				}
				_, err := buf.Write(line)
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
