// Copyright 2019 Cloudbase Solutions SRL

package logging

type Writer interface {
	Write(logMsg LogMessage) error
}
