package stdout

import (
	"fmt"
	"time"

	"github.com/gabriel-samfira/coriolis-logger/datastore"
)

func NewStdOutDatastore() (datastore.DataStore, error) {
	return &StdOutDataStore{}, nil
}

var _ datastore.DataStore = (*StdOutDataStore)(nil)

// StdOutDataStore is a dummy datastore that simply writes
// to standard output. It should only be used for testing
// and debugging purposes.
type StdOutDataStore struct{}

func (i *StdOutDataStore) Write(logParts map[string]interface{}) error {
	if data, ok := logParts["content"]; ok {
		fmt.Println(data)

	} else if data, ok := logParts["message"]; ok {
		fmt.Println(data)
	}
	return nil
}

func (i *StdOutDataStore) Rotate(olderThan time.Duration) error {
	return nil
}
