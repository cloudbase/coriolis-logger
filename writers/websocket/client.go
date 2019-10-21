// Copyright 2019 Cloudbase Solutions SRL

package websocket

import (
	"time"

	"github.com/google/uuid"

	"github.com/gabriel-samfira/coriolis-logger/logging"
	"github.com/gorilla/websocket"
	"github.com/juju/loggo"
)

var log = loggo.GetLogger("coriolis.apiserver.client")

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 1024
)

type ClientFilterOptions struct {
	Severity *logging.Severity `json:"omitempty"`
	AppName  *string
}

func NewClient(conn *websocket.Conn, opts ClientFilterOptions, hub *Hub) (*Client, error) {
	clientID := uuid.New()
	return &Client{
		id:      clientID.String(),
		options: opts,
		conn:    conn,
		hub:     hub,
		send:    make(chan LogMessage, 1024),
	}, nil
}

type Client struct {
	id      string
	options ClientFilterOptions
	conn    *websocket.Conn
	// Buffered channel of outbound messages.
	send chan LogMessage

	hub *Hub
}

func (c *Client) Go() {
	go c.clientReader()
	go c.clientWriter()
}

// clientReader waits for options changes from the client. The client can at any time
// change the log level and binary name it watches.
func (c *Client) clientReader() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		opts := ClientFilterOptions{}
		if err := c.conn.ReadJSON(&opts); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Errorf("error: %v", err)
			}
			break
		}
		c.options = opts
	}
}

// clientWriter
func (c *Client) clientWriter() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(message); err != nil {
				log.Errorf("error sending message: %v", err)
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) ShouldSend(msg logging.LogMessage) bool {
	severity := logging.DefaultSeverityLevel
	var binName string
	if c.options.Severity != nil {
		severity = *c.options.Severity
	}

	if c.options.AppName != nil {
		binName = *c.options.AppName
	}

	if binName != "" && binName != msg.AppName {
		return false
	}
	if msg.Severity > severity {
		return false
	}
	return true
}

func (c *Client) SyslogMessageToLogMessage(msg logging.LogMessage) LogMessage {
	return LogMessage{
		Severity:  int(msg.Severity),
		AppName:   msg.AppName,
		Hostname:  msg.Hostname,
		Timestamp: msg.Timestamp,
		Message:   msg.Message,
	}
}
