package apiserver

import (
	"fmt"
	"time"

	"github.com/gabriel-samfira/coriolis-logger/logging"
)

func NewHub() *Hub {
	return &Hub{
		clients:    map[string]*Client{},
		broadcast:  make(chan logging.LogMessage, 100),
		register:   make(chan *Client, 100),
		unregister: make(chan *Client, 100),
	}
}

type Hub struct {
	// Registered clients.
	clients map[string]*Client

	// Inbound messages from the clients.
	broadcast chan logging.LogMessage

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			if client != nil {
				h.clients[client.id] = client
			}
		case client := <-h.unregister:
			if client != nil {
				if _, ok := h.clients[client.id]; ok {
					delete(h.clients, client.id)
					close(client.send)
				}
			}
		case message := <-h.broadcast:
			for id, client := range h.clients {
				if client == nil {
					continue
				}
				if !client.ShouldSend(message) {
					continue
				}
				asBytes, err := client.LogMessageToBytes(message)
				if err != nil {
					continue
				}
				select {
				case client.send <- asBytes:
				default:
					close(client.send)
					delete(h.clients, id)
				}
			}
		}
	}
}

func (h *Hub) Write(msg logging.LogMessage) error {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	select {
	case <-ticker.C:
		return fmt.Errorf("timed out sending message to client")
	case h.broadcast <- msg:
	}
	return nil
}
