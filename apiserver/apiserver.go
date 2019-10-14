package apiserver

import (
	"github.com/gorilla/websocket"
)

type LogMessage struct {
	Severity int
	Binary   string
	Message  string
}

type Client struct {
	options map[string]string
	conn    *websocket.Conn
}

type Hub struct {
	clients map[*Client]bool
}

func (h *Hub) Register(client *Client) error {
	return nil
}

func (h *Hub) Unregister(client *Client) error {
	return nil
}

func (h *Hub) Broadcast(msg LogMessage) error {
	return nil
}
