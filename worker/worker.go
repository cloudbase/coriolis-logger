package worker

type SimpleWorker interface {
	Start() error
	Stop() error
	Wait()
}
