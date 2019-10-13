package syslog

import (
	"context"
	"fmt"

	syslog "gopkg.in/mcuadros/go-syslog.v2"

	"github.com/gabriel-samfira/coriolis-logger/config"
	"github.com/gabriel-samfira/coriolis-logger/datastore"
	"github.com/gabriel-samfira/coriolis-logger/datastore/influxdb"
	"github.com/gabriel-samfira/coriolis-logger/datastore/stdout"
	"github.com/gabriel-samfira/coriolis-logger/worker"
	"github.com/pkg/errors"
)

func getDatastore(cfg config.Syslog) (datastore.DataStore, error) {
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
	case config.StdOutDataStore:
		return stdout.NewStdOutDatastore()
	default:
		return nil, fmt.Errorf("invalid datastore type")
	}
}

func NewSyslogServer(ctx context.Context, cfg config.Syslog, errChan chan error) (worker.SimpleWorker, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(err, "validating syslog config")
	}
	store, err := getDatastore(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "fetching datastore")
	}

	channel := make(syslog.LogPartsChannel)
	handler := syslog.NewChannelHandler(channel)
	server := syslog.NewServer()
	logFormat, err := cfg.LogFormat()
	if err != nil {
		return nil, errors.Wrap(err, "getting log format")
	}
	server.SetFormat(logFormat)
	server.SetHandler(handler)

	switch cfg.Listener {
	case config.UnixDgramListener:
		if err := server.ListenUnixgram(cfg.Address); err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("listening on unix socket %q", cfg.Address))
		}
	case config.TCPListener:
		if err := server.ListenTCP(cfg.Address); err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("listening on TCP %q", cfg.Address))
		}
	case config.UDPListener:
		if err := server.ListenUDP(cfg.Address); err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("listening on UDP %q", cfg.Address))
		}
	}

	return &SyslogWorker{
		server:    server,
		datastore: store,
		cfg:       cfg,
		channel:   channel,
		ctx:       ctx,
		errChan:   errChan,
	}, nil
}

var _ worker.SimpleWorker = (*SyslogWorker)(nil)

type SyslogWorker struct {
	datastore datastore.DataStore
	cfg       config.Syslog
	server    *syslog.Server
	channel   syslog.LogPartsChannel
	ctx       context.Context
	errChan   chan error
}

func (s *SyslogWorker) doWork() {
	for {
		select {
		case logParts, ok := <-s.channel:
			if !ok {
				// channel was closed, exiting
				return
			}
			if err := s.datastore.Write(logParts); err != nil {
				// An error was returned by the datastore. We should exit
				// and allow the process manager or the operator to restart
				// the syslog server.
				s.errChan <- err
				return
			}
		case <-s.ctx.Done():
			close(s.channel)
			return
		}
	}
}

func (s *SyslogWorker) Start() error {
	err := s.server.Boot()
	if err != nil {
		return errors.Wrap(err, "starting syslog server")
	}
	go s.doWork()
	return nil
}

func (s *SyslogWorker) Stop() error {
	if _, ok := <-s.channel; ok {
		close(s.channel)
	}
	return s.server.Kill()
}
