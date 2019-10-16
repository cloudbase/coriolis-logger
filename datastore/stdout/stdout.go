package stdout

import (
	"fmt"
	"time"

	"github.com/gabriel-samfira/coriolis-logger/datastore/common"
	"github.com/gabriel-samfira/coriolis-logger/logging"
)

func NewStdOutDatastore() (common.DataStore, error) {
	return &StdOutDataStore{}, nil
}

var _ common.DataStore = (*StdOutDataStore)(nil)

// StdOutDataStore is a dummy datastore that simply writes
// to standard output. It should only be used for testing
// and debugging purposes.
type StdOutDataStore struct{}

func (i *StdOutDataStore) Write(logMsg logging.LogMessage) error {
	fmt.Println(logMsg.BinaryName, logMsg.Message)
	return nil
}

func (i *StdOutDataStore) Rotate(olderThan time.Time) error {
	return nil
}

func (i *StdOutDataStore) Read(start, end time.Time) ([]byte, error) {
	return nil, nil
}
