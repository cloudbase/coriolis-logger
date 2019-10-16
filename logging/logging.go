package logging

import (
	"github.com/juju/loggo"
	"github.com/pkg/errors"
)

var log = loggo.GetLogger("coriolis-logger.logging")

type aggregateWriter struct {
	writers []Writer
}

func NewAggregateWriter(writer ...Writer) Writer {
	wr := &aggregateWriter{
		writers: writer,
	}
	return wr
}

func (a *aggregateWriter) Write(msg LogMessage) (err error) {
	errs := []error{}
	defer func() {
		if len(errs) > 0 {
			// more than one error may occur. We should return them all
			err = errors.Wrap(errs[0], "writing log message")
		}
		return
	}()
	for _, val := range a.writers {
		go func(w Writer) {
			if err := w.Write(msg); err != nil {
				errs = append(errs, err)
				log.Errorf("failed to write log message: %q", err)
			}
		}(val)
	}
	return
}
