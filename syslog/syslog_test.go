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
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"coriolis-logger/config"
	"coriolis-logger/logging"
)

type collectingWriter struct {
	mu   sync.Mutex
	msgs []logging.LogMessage
}

func (c *collectingWriter) Write(logMsg logging.LogMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, logMsg)
	return nil
}

func (c *collectingWriter) messages() []logging.LogMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]logging.LogMessage, len(c.msgs))
	copy(out, c.msgs)
	return out
}

func freeListenAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving tcp port: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("releasing tcp port: %v", err)
	}
	return addr
}

func syslogRFC5424(msg string) string {
	return fmt.Sprintf("<14>1 2024-01-02T03:04:05Z testhost coriolis-logger - - - %s", msg)
}

func sendUnixgram(t *testing.T, socket, msg string) {
	t.Helper()
	conn, err := net.Dial("unixgram", socket)
	if err != nil {
		t.Fatalf("dial unixgram: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(msg)); err != nil {
		t.Fatalf("write unixgram: %v", err)
	}
}

func sendTCP(t *testing.T, addr, msg string) {
	t.Helper()
	var conn net.Conn
	var err error
	deadline := time.Now().Add(2 * time.Second)
	for {
		conn, err = net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("dial tcp: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(msg + "\n")); err != nil {
		t.Fatalf("write tcp: %v", err)
	}
}

func waitForMessages(t *testing.T, writer *collectingWriter, n int) []logging.LogMessage {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		msgs := writer.messages()
		if len(msgs) >= n {
			return msgs
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d messages, got %d", n, len(writer.messages()))
	return nil
}

func TestSyslogWorkerUnixAndTCP(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "syslog.sock")
	tcpAddr := freeListenAddr(t)

	cfg := config.Syslog{
		Listener:      config.UnixDgramListener,
		Address:       socket,
		ExtraListener: config.TCPListener,
		ExtraAddress:  tcpAddr,
		Format:        "rfc5424",
		DataStore:     config.StdOutDataStore,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writer := &collectingWriter{}
	worker, err := NewSyslogServer(ctx, cfg, writer, make(chan error, 1))
	if err != nil {
		t.Fatalf("creating syslog server: %v", err)
	}
	if err := worker.Start(); err != nil {
		t.Fatalf("starting syslog server: %v", err)
	}
	defer func() {
		cancel()
		worker.Wait()
	}()

	sendUnixgram(t, socket, syslogRFC5424("from-unix"))
	sendTCP(t, tcpAddr, syslogRFC5424("from-tcp"))

	msgs := waitForMessages(t, writer, 2)
	seen := map[string]bool{}
	for _, msg := range msgs {
		seen[msg.Message] = true
	}
	if !seen["from-unix"] {
		t.Errorf("missing unix datagram message, got %+v", msgs)
	}
	if !seen["from-tcp"] {
		t.Errorf("missing tcp message, got %+v", msgs)
	}
}
