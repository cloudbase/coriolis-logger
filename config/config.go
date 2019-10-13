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
	"github.com/pkg/errors"
	"gopkg.in/mcuadros/go-syslog.v2"
	"gopkg.in/mcuadros/go-syslog.v2/format"
)

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
	CRT    string
	Key    string
	CACert string
}

func (t *TLSConfig) TLSConfig() (*tls.Config, error) {
	// TLS config not present.
	if t.CRT == "" && t.Key == "" {
		return nil, fmt.Errorf("missing crt or key")
	}

	var roots *x509.CertPool
	if t.CACert != "" {
		caCertPEM, err := ioutil.ReadFile(t.CACert)
		if err != nil {
			return nil, err
		}
		roots = x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(caCertPEM)
		if !ok {
			return nil, fmt.Errorf("failed to parse CA cert")
		}
	}

	cert, err := tls.LoadX509KeyPair(t.CRT, t.Key)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    roots,
	}, nil
}

func (t *TLSConfig) Validate() error {
	if _, err := t.TLSConfig(); err != nil {
		return err
	}
	return nil
}

// APIServer holds configuration for the API server
// worker
type APIServer struct {
	Bind      string
	Port      int
	TLSConfig TLSConfig `toml:"tls"`
}

func (a *APIServer) Validate() error {
	if err := a.TLSConfig.Validate(); err != nil {
		return errors.Wrap(err, "TLS validation failed")
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
	Listener  ListenerType
	Address   string
	Format    string
	DataStore DatastoreType
	InfluxDB  *InfluxDB `toml:"influxdb"`
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
	URL          InfluxURL `toml:"url"`
	Username     string
	Password     string
	VerifyServer bool
	CACert       string
	ClientCRT    string
	ClientKey    string
}

func (i *InfluxDB) Validate() error {
	if i.URL.IsValid() == false {
		return fmt.Errorf("invalid InfluxDB URL: %q", i.URL)
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
