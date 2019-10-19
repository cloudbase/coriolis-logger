package stdout

import (
	"github.com/gabriel-samfira/coriolis-logger/logging"
)

func NewStdOutWriter() (logging.Writer, error) {
	return &StdOutWriter{}, nil
}

var _ logging.Writer = (*StdOutWriter)(nil)

// StdOutWriter is a simple writer that writes to stdout
type StdOutWriter struct{}

func (i *StdOutWriter) Write(logMsg logging.LogMessage) error {
	// fmt.Println(logMsg.Message)
	return nil
}
