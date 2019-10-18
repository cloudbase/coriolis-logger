package common

import (
	"time"

	"github.com/gabriel-samfira/coriolis-logger/logging"
	"github.com/gabriel-samfira/coriolis-logger/params"
	client "github.com/influxdata/influxdb1-client/v2"
)

type DataStore interface {
	Write(logMsg logging.LogMessage) error
	Rotate(olderThan time.Time) error
	ResultReader(p params.QueryParams) Reader
	Query(q client.Query) (*client.ChunkedResponse, error)
}

type Reader interface {
	ReadNext() ([]byte, error)
}
