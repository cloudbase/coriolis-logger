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

package config

import (
	"path/filepath"
	"testing"
)

func validSyslog(t *testing.T) Syslog {
	t.Helper()
	return Syslog{
		Listener:  UnixDgramListener,
		Address:   filepath.Join(t.TempDir(), "syslog.sock"),
		Format:    "automatic",
		DataStore: StdOutDataStore,
	}
}

func TestSyslogValidateExtraListener(t *testing.T) {
	t.Parallel()

	t.Run("primary only", func(t *testing.T) {
		cfg := validSyslog(t)
		if err := cfg.Validate(); err != nil {
			t.Fatalf("expected valid config, got %v", err)
		}
	})

	t.Run("unix and tcp", func(t *testing.T) {
		cfg := validSyslog(t)
		cfg.ExtraListener = TCPListener
		cfg.ExtraAddress = "127.0.0.1:5144"
		if err := cfg.Validate(); err != nil {
			t.Fatalf("expected valid unix+tcp config, got %v", err)
		}
	})

	t.Run("unix and udp", func(t *testing.T) {
		cfg := validSyslog(t)
		cfg.ExtraListener = UDPListener
		cfg.ExtraAddress = "127.0.0.1:5144"
		if err := cfg.Validate(); err != nil {
			t.Fatalf("expected valid unix+udp config, got %v", err)
		}
	})

	t.Run("tcp primary with unix extra", func(t *testing.T) {
		cfg := validSyslog(t)
		cfg.Listener = TCPListener
		cfg.Address = "0.0.0.0:5144"
		cfg.ExtraListener = UnixDgramListener
		cfg.ExtraAddress = filepath.Join(t.TempDir(), "extra.sock")
		if err := cfg.Validate(); err != nil {
			t.Fatalf("expected valid tcp+unix config, got %v", err)
		}
	})

	t.Run("extra listener without address", func(t *testing.T) {
		cfg := validSyslog(t)
		cfg.ExtraListener = TCPListener
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error when extra_address is missing")
		}
	})

	t.Run("extra address without listener", func(t *testing.T) {
		cfg := validSyslog(t)
		cfg.ExtraAddress = "127.0.0.1:5144"
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error when extra_listener is missing")
		}
	})

	t.Run("duplicate listener", func(t *testing.T) {
		cfg := validSyslog(t)
		cfg.Listener = TCPListener
		cfg.Address = "127.0.0.1:5144"
		cfg.ExtraListener = TCPListener
		cfg.ExtraAddress = "127.0.0.1:5144"
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error when extra listener duplicates the primary")
		}
	})

	t.Run("invalid extra listener", func(t *testing.T) {
		cfg := validSyslog(t)
		cfg.ExtraListener = "sctp"
		cfg.ExtraAddress = "127.0.0.1:5144"
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for invalid extra_listener")
		}
	})
}
