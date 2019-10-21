package websocket

import "time"

type LogMessage struct {
	Severity  int       `json:"severity"`
	AppName   string    `json:"app_name"`
	Message   string    `json:"message"`
	Hostname  string    `json:"hostname"`
	Timestamp time.Time `json:"timestamp"`
}
