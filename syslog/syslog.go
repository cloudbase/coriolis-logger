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

package syslog

import (
	"context"
	"fmt"
	"os"

	syslog "gopkg.in/mcuadros/go-syslog.v2"

	"coriolis-logger/config"
	"coriolis-logger/logging"
	"coriolis-logger/worker"

	"github.com/juju/loggo"
	"github.com/pkg/errors"
)

var log = loggo.GetLogger("coriolis.logger.syslog")

func init() {
	log.SetLogLevel(loggo.DEBUG)
}

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

	worker := &SyslogWorker{
		server:  server,
		logging: writer,
		cfg:     cfg,
		channel: channel,
		ctx:     ctx,
		errChan: errChan,
		closed:  make(chan struct{}),
	}

	return worker, nil
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
			}
		case <-s.ctx.Done():
			s.Stop()
			return
		}
	}
}

func (s *SyslogWorker) Start() error {
	if err := s.cleanStaleSocket(); err != nil {
		return errors.Wrap(err, "removing socket")
	}

	switch s.cfg.Listener {
	case config.UnixDgramListener:
		if err := s.server.ListenUnixgram(s.cfg.Address); err != nil {
			return errors.Wrap(err, fmt.Sprintf("listening on unix socket %q", s.cfg.Address))
		}
		if _, err := os.Stat(s.cfg.Address); err != nil {
			log.Warningf("cannot fetch info about %q: %q", s.cfg.Address, err)
		} else {
			if err := os.Chmod(s.cfg.Address, 0666); err != nil {
				log.Warningf("cannot change permissions on %q: %q", s.cfg.Address, err)
			}
		}
	case config.TCPListener:
		if err := s.server.ListenTCP(s.cfg.Address); err != nil {
			return errors.Wrap(err, fmt.Sprintf("listening on TCP %q", s.cfg.Address))
		}
	case config.UDPListener:
		if err := s.server.ListenUDP(s.cfg.Address); err != nil {
			return errors.Wrap(err, fmt.Sprintf("listening on UDP %q", s.cfg.Address))
		}
	}

	err := s.server.Boot()
	if err != nil {
		return errors.Wrap(err, "starting syslog server")
	}
	go s.doWork()
	return nil
}

func (s *SyslogWorker) cleanStaleSocket() error {
	if s.cfg.Listener != config.UnixDgramListener {
		return nil
	}
	if mode, err := os.Stat(s.cfg.Address); err == nil {
		if mode.Mode()&os.ModeSocket != 0 {
			log.Infof("removing unix socket %q", s.cfg.Address)
			if err := os.Remove(s.cfg.Address); err != nil {
				return errors.Wrap(err, "removing unix socket")
			}
		}
	}
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
	if err := s.cleanStaleSocket(); err != nil {
		return errors.Wrap(err, "removing socket")
	}
	return nil
}

func (s *SyslogWorker) Wait() {
	<-s.closed
}
