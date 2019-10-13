package datastore

import "time"

type DataStore interface {
	Write(logParts map[string]interface{}) error
	Rotate(olderThan time.Duration) error
}
