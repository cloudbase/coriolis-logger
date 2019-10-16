package common

import (
	"time"

	"github.com/gabriel-samfira/coriolis-logger/logging"
)

type DataStore interface {
	Write(logMsg logging.LogMessage) error
	Rotate(olderThan time.Time) error
}
