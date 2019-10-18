package params

import "time"

// QueryParams represents log filter parameters for log readers
type QueryParams struct {
	Hostname   string
	StartDate  time.Time
	EndDate    time.Time
	BinaryName string
	Severity   int
}
