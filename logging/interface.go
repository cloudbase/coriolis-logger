package logging

type Writer interface {
	Write(logMsg LogMessage) error
}
