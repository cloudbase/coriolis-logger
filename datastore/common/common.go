package common

import (
	"time"

	"github.com/gabriel-samfira/coriolis-logger/logging"
	"github.com/gabriel-samfira/coriolis-logger/params"
	"github.com/gabriel-samfira/coriolis-logger/worker"
	client "github.com/influxdata/influxdb1-client/v2"
)

type DataStore interface {
	worker.SimpleWorker

	Write(logMsg logging.LogMessage) error
	Rotate(olderThan time.Time) error
	ResultReader(p params.QueryParams) Reader
	List() ([]string, error)
	Query(q client.Query) (*client.ChunkedResponse, error)
}

type Reader interface {
	ReadNext() ([]byte, error)
}
