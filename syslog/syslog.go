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

	if err := s.listen(s.cfg.Listener, s.cfg.Address); err != nil {
		return err
	}
	if s.cfg.ExtraListener != "" {
		if err := s.listen(s.cfg.ExtraListener, s.cfg.ExtraAddress); err != nil {
			return err
		}
	}

	err := s.server.Boot()
	if err != nil {
		return errors.Wrap(err, "starting syslog server")
	}
	go s.doWork()
	return nil
}

func (s *SyslogWorker) listen(listener config.ListenerType, address string) error {
	switch listener {
	case config.UnixDgramListener:
		if err := s.server.ListenUnixgram(address); err != nil {
			return errors.Wrap(err, fmt.Sprintf("listening on unix socket %q", address))
		}
		if _, err := os.Stat(address); err != nil {
			log.Warningf("cannot fetch info about %q: %q", address, err)
		} else {
			if err := os.Chmod(address, 0666); err != nil {
				log.Warningf("cannot change permissions on %q: %q", address, err)
			}
		}
	case config.TCPListener:
		if err := s.server.ListenTCP(address); err != nil {
			return errors.Wrap(err, fmt.Sprintf("listening on TCP %q", address))
		}
	case config.UDPListener:
		if err := s.server.ListenUDP(address); err != nil {
			return errors.Wrap(err, fmt.Sprintf("listening on UDP %q", address))
		}
	default:
		return fmt.Errorf("invalid listener type %q", listener)
	}
	return nil
}

func (s *SyslogWorker) unixAddresses() []string {
	var addrs []string
	if s.cfg.Listener == config.UnixDgramListener {
		addrs = append(addrs, s.cfg.Address)
	}
	if s.cfg.ExtraListener == config.UnixDgramListener {
		addrs = append(addrs, s.cfg.ExtraAddress)
	}
	return addrs
}

func (s *SyslogWorker) cleanStaleSocket() error {
	for _, addr := range s.unixAddresses() {
		if mode, err := os.Stat(addr); err == nil {
			if mode.Mode()&os.ModeSocket != 0 {
				log.Infof("removing unix socket %q", addr)
				if err := os.Remove(addr); err != nil {
					return errors.Wrap(err, "removing unix socket")
				}
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
