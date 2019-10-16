package syslog

import (
	"context"
	"fmt"
	"os"

	syslog "gopkg.in/mcuadros/go-syslog.v2"

	"github.com/gabriel-samfira/coriolis-logger/config"
	"github.com/gabriel-samfira/coriolis-logger/logging"
	"github.com/gabriel-samfira/coriolis-logger/worker"
	"github.com/juju/loggo"
	"github.com/pkg/errors"
)

var log = loggo.GetLogger("coriolis.logger.syslog")

func init() {
	log.SetLogLevel(loggo.DEBUG)
}

// func getDatastore(cfg config.Syslog) (datastore.DataStore, error) {
// 	if err := cfg.Validate(); err != nil {
// 		return nil, errors.Wrap(err, "validating syslog config")
// 	}
// 	switch cfg.DataStore {
// 	case config.InfluxDBDatastore:
// 		// Validation should already be done by the config package, but
// 		// it pays to be paranoid sometimes
// 		if cfg.InfluxDB == nil {
// 			return nil, fmt.Errorf("invalid influxdb datastore config")
// 		}
// 		return influxdb.NewInfluxDBDatastore(cfg.InfluxDB)
// 	case config.StdOutDataStore:
// 		return stdout.NewStdOutDatastore()
// 	default:
// 		return nil, fmt.Errorf("invalid datastore type")
// 	}
// }

func NewSyslogServer(ctx context.Context, cfg config.Syslog, writer logging.Writer, errChan chan error) (worker.SimpleWorker, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(err, "validating syslog config")
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
		server:  server,
		logging: writer,
		cfg:     cfg,
		channel: channel,
		ctx:     ctx,
		errChan: errChan,
		closed:  make(chan struct{}),
	}, nil
}

var _ worker.SimpleWorker = (*SyslogWorker)(nil)

type SyslogWorker struct {
	logging logging.Writer
	cfg     config.Syslog
	server  *syslog.Server
	channel syslog.LogPartsChannel
	ctx     context.Context
	errChan chan error
	closed  chan struct{}
}

func (s *SyslogWorker) doWork() {
	for {
		select {
		case logParts, ok := <-s.channel:
			if !ok {
				// channel was closed, exiting
				return
			}
			logMsg, err := logging.SyslogToLogMessage(logParts)
			if err != nil {
				log.Errorf("failed to parse log message: %q", err)
				continue
			}
			if err := s.logging.Write(logMsg); err != nil {
				log.Errorf("failed to write log message: %q", err)
				continue
				// TODO (gsamfira): decide whether we want to stop the server
				// when an error occurs here.
				// s.errChan <- err
				// s.Stop()
				// return
			}
		case <-s.ctx.Done():
			s.Stop()
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
	log.Infof("stopping syslog worker")
	defer close(s.closed)
	select {
	case _, ok := <-s.channel:
		if ok {
			close(s.channel)
		}
	default:
		close(s.channel)
	}
	if err := s.server.Kill(); err != nil {
		return errors.Wrap(err, "killing syslog server")
	}
	if s.cfg.Listener == config.UnixDgramListener {
		if mode, err := os.Stat(s.cfg.Address); err == nil {
			if mode.Mode()&os.ModeSocket != 0 {
				if err := os.Remove(s.cfg.Address); err != nil {
					return errors.Wrap(err, "removing unix socket")
				}
			}
		}
	}
	return nil
}

func (s *SyslogWorker) Wait() {
	<-s.closed
}
