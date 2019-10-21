// Copyright 2019 Cloudbase Solutions SRL

package worker

type SimpleWorker interface {
	Start() error
	Stop() error
	Wait()
}
