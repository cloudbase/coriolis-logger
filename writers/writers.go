package writers

import (
	"github.com/gabriel-samfira/coriolis-logger/logging"
	"github.com/gabriel-samfira/coriolis-logger/writers/stdout"
)

func NewStdOutWriter() (logging.Writer, error) {
	return &stdout.StdOutWriter{}, nil
}
