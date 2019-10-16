package apiserver

import "time"

type LogMessage struct {
	Severity  int       `json:"severity"`
	Binary    string    `json:"binary_name"`
	Message   string    `json:"message"`
	Hostname  string    `json:"hostname"`
	Timestamp time.Time `json:"timestamp"`
}
