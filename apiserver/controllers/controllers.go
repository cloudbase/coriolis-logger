package controllers

import (
	"net/http"
	"strconv"

	"github.com/gabriel-samfira/coriolis-logger/logging"
	wsWriter "github.com/gabriel-samfira/coriolis-logger/writers/websocket"
	"github.com/gorilla/websocket"
	"github.com/juju/loggo"
)

var log = loggo.GetLogger("coriolis.logger.controllers")

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 16384,
}

func NewLogHandler(hub *wsWriter.Hub) *LogHandlers {
	return &LogHandlers{
		hub: hub,
	}
}

type LogHandlers struct {
	hub *wsWriter.Hub
	
}

func (l *LogHandlers) WSHandler(writer http.ResponseWriter, req *http.Request) {
	var severity logging.Severity
	binName := req.URL.Query().Get("binary_name")
	cliSeverityStr := req.URL.Query().Get("severity")
	if cliSeverityStr != "" {
		cliSeverity, err := strconv.Atoi(cliSeverityStr)
		if err != nil {
			log.Warningf("invalid severity %q. Ignoring", cliSeverityStr)
		}
		if cliSeverity > 7 || cliSeverity < 0 {
			severity = logging.DefaultSeverityLevel
		} else {
			severity = logging.Severity(cliSeverity)
		}
	}

	conn, err := upgrader.Upgrade(writer, req, nil)
	if err != nil {
		log.Errorf("error upgrading to websockets: %v", err)
		return
	}

	opts := wsWriter.ClientFilterOptions{
		Severity:   &severity,
		BinaryName: &binName,
	}
	client, err := wsWriter.NewClient(conn, opts, l.hub)
	if err != nil {
		log.Errorf("failed to create new client: %v", err)
		return
	}
	if err := l.hub.Register(client); err != nil {
		log.Errorf("failed to register new client: %v", err)
		return
	}
	client.Go()
}

func (l *LogHandlers) DownloadLogHandler(writer http.ResponseWriter, req *http.Request) {
	return
}
