package apiserver

import (
	"encoding/json"

	"github.com/gabriel-samfira/coriolis-logger/logging"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type ClientFilterOptions struct {
	Severity   logging.Severity
	BinaryName string
}

type Client struct {
	id      string
	options *ClientFilterOptions
	conn    *websocket.Conn
	// Buffered channel of outbound messages.
	send chan []byte
}

func (c *Client) ShouldSend(msg logging.LogMessage) bool {
	severity := logging.Informational
	var binName string
	if c.options != nil {
		severity = c.options.Severity
		binName = c.options.BinaryName
	}
	if binName != "" && binName != msg.BinaryName {
		return false
	}
	if msg.Severity > severity {
		return false
	}
	return true
}

func (c *Client) LogMessageToBytes(msg logging.LogMessage) ([]byte, error) {
	clientMsg := LogMessage{
		Severity:  int(msg.Severity),
		Binary:    msg.BinaryName,
		Hostname:  msg.Hostname,
		Timestamp: msg.Timestamp,
	}

	asBytes, err := json.Marshal(clientMsg)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling message")
	}
	return asBytes, nil
}
