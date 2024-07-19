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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/juju/loggo"
	"github.com/pkg/errors"
	"gopkg.in/mcuadros/go-syslog.v2"
	"gopkg.in/mcuadros/go-syslog.v2/format"
)

var log = loggo.GetLogger("coriolis.logger.config")

// DatastoreType represents the datastore type the syslog
// worker can use to save logs
type DatastoreType string

// ListenerType represents the listener types available
// for the syslog worker
type ListenerType string

const (
	UnixDgramListener ListenerType = "unixgram"
	TCPListener       ListenerType = "tcp"
	UDPListener       ListenerType = "udp"

	InfluxDBDatastore DatastoreType = "influxdb"
	StdOutDataStore   DatastoreType = "stdout"

	DefaultConfigDir  = "/etc/coriolis-logger"
	DefaultConfigFile = "/etc/coriolis-logger/coriolis-logger.toml"

	AuthenticationKeystone = "keystone"
	AuthenticationNone     = "none"

	DefaultLogRetentionPeriod = 3
)

// NewConfig returns a new Config
func NewConfig(cfgFile string) (*Config, error) {
	var config Config
	if _, err := toml.DecodeFile(cfgFile, &config); err != nil {
		return nil, err
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &config, nil
}

// TLSConfig is the API server TLS config
type TLSConfig struct {
	CRT string
	Key string
}

func (t *TLSConfig) Validate() error {
	if t.CRT == "" || t.Key == "" {
		return fmt.Errorf("missing crt or key")
	}

	if _, err := os.Stat(t.CRT); err != nil {
		return errors.Wrapf(err, "failed to access %s", t.CRT)
	}

	if _, err := os.Stat(t.Key); err != nil {
		return errors.Wrapf(err, "failed to access %s", t.Key)
	}

	_, err := tls.LoadX509KeyPair(t.CRT, t.Key)
	if err != nil {
		return errors.Wrap(err, "loading X509 key pair")
	}
	return nil
}

type KeystoneAuth struct {
	AuthURI    string   `toml:"auth_uri"`
	AdminRoles []string `toml:"admin_roles"`
}

func (k *KeystoneAuth) Validate() error {
	if k.AuthURI == "" {
		return fmt.Errorf("missing keystone auth_uri")
	}
	return nil
}

// APIServer holds configuration for the API server
// worker
type APIServer struct {
	Bind           string
	Port           int
	UseTLS         bool
	AuthMiddleware string        `toml:"auth_middleware"`
	TLSConfig      TLSConfig     `toml:"tls"`
	KeystoneAuth   *KeystoneAuth `toml:"keystone_auth"`
	CORSOrigins    []string      `toml:"cors_origins"`
}

func (a *APIServer) Validate() error {
	switch a.AuthMiddleware {
	case AuthenticationKeystone:
		if a.KeystoneAuth == nil {
			return fmt.Errorf("keystone authentication enabled, but missing keystone config section")
		}
		if err := a.KeystoneAuth.Validate(); err != nil {
			return errors.Wrap(err, "validating keystone config")
		}
	case AuthenticationNone:
		log.Warningf("authentication is disabled. Anyone can view your logs!")
	default:
		return fmt.Errorf("no authentication is enabled")
	}

	if a.UseTLS {
		if err := a.TLSConfig.Validate(); err != nil {
			return errors.Wrap(err, "TLS validation failed")
		}
	}
	if a.Port > 65535 || a.Port < 1 {
		return fmt.Errorf("invalid port nr %q", a.Port)
	}
	ip := net.ParseIP(a.Bind)
	if ip == nil {
		// No need for deeper validation here, as any invalid
		// IP address specified in this setting will raise an error
		// when we try to bind to it.
		return fmt.Errorf("invalid IP address")
	}
	return nil
}

type Syslog struct {
	Listener    ListenerType
	Address     string
	Format      string
	LogToStdout bool `toml:"log_to_stdout"`
	DataStore   DatastoreType
	InfluxDB    *InfluxDB `toml:"influxdb"`
}

func (s *Syslog) LogFormat() (format.Format, error) {
	switch s.Format {
	case "automatic":
		return syslog.Automatic, nil
	case "rfc3164":
		return syslog.RFC3164, nil
	case "rfc5424":
		return syslog.RFC5424, nil
	case "rfc6587":
		return syslog.RFC6587, nil
	default:
		return nil, fmt.Errorf("invalid log format %q", s.Format)
	}
}

func (s *Syslog) Validate() error {
	switch s.DataStore {
	case InfluxDBDatastore:
		if s.InfluxDB == nil {
			return fmt.Errorf("no influxdb config found")
		}
		if err := s.InfluxDB.Validate(); err != nil {
			return errors.Wrap(err, "validating influxdb")
		}
	case StdOutDataStore:
	default:
		return fmt.Errorf("invalid datastore type %q", s.DataStore)
	}

	switch s.Listener {
	case UnixDgramListener:
		absPath, err := filepath.Abs(s.Address)
		if err != nil {
			return errors.Wrap(err, "getting dirname")
		}
		parent := filepath.Dir(absPath)
		if _, err := os.Stat(parent); err != nil {
			return errors.Wrap(err, "fetching info about dirname")
		}

		if mode, err := os.Stat(s.Address); err == nil {
			if mode.Mode()&os.ModeSocket == 0 {
				return fmt.Errorf(
					"cannot use %q as address. File already exists and is not socket", s.Address)
			}
		}
	case TCPListener, UDPListener:
	default:
		return fmt.Errorf("invalid listener type %q", s.Listener)
	}
	return nil
}

// InfluxURL represents an influxDB URL
type InfluxURL string

func (i InfluxURL) String() string {
	return string(i)
}

func (i InfluxURL) IsValid() bool {
	url, err := url.Parse(string(i))
	if err != nil {
		return false
	}
	if url.Scheme != "http" && url.Scheme != "https" {
		return false
	}

	if url.Host == "" {
		return false
	}
	return true
}

func (i InfluxURL) IsSSL() bool {
	url, err := url.Parse(string(i))
	if err != nil {
		return false
	}
	if url.Scheme == "https" {
		return true
	}
	return false
}

// InfluxDB holds the influxdb credentials
type InfluxDB struct {
	URL                InfluxURL `toml:"url"`
	Username           string
	Password           string
	Database           string
	VerifyServer       bool
	CACert             string
	ClientCRT          string
	ClientKey          string
	WriteInterval      int `toml:"write_interval"`
	LogRetentionPeriod int `toml:"log_retention_period"`
}

func (i InfluxDB) GetLogRetention() int {
	if i.LogRetentionPeriod == 0 {
		return DefaultLogRetentionPeriod
	}
	return i.LogRetentionPeriod
}

func (i *InfluxDB) TLSConfig() (*tls.Config, error) {
	if i.CACert == "" && i.ClientCRT == "" && i.ClientKey == "" {
		return nil, nil
	}

	cfg := &tls.Config{}

	var roots *x509.CertPool
	if i.CACert != "" {
		caCertPEM, err := ioutil.ReadFile(i.CACert)
		if err != nil {
			return nil, err
		}
		roots = x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(caCertPEM)
		if !ok {
			return nil, fmt.Errorf("failed to parse CA cert")
		}
		cfg.ClientCAs = roots
	}

	if i.ClientKey != "" && i.ClientCRT != "" {
		cert, err := tls.LoadX509KeyPair(i.ClientCRT, i.ClientKey)
		if err != nil {
			return nil, err
		}
		cfg.Certificates = []tls.Certificate{cert}
	}

	if i.VerifyServer {
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return cfg, nil
}

func (i *InfluxDB) Validate() error {
	if !i.URL.IsValid() {
		return fmt.Errorf("invalid InfluxDB URL: %q", i.URL)
	}
	if i.Database == "" {
		return fmt.Errorf("invalid database name")
	}
	return nil
}

type Config struct {
	APIServer APIServer
	Syslog    Syslog
}

func (c *Config) Validate() error {
	if err := c.APIServer.Validate(); err != nil {
		return err
	}

	if err := c.Syslog.Validate(); err != nil {
		return err
	}
	return nil
}
