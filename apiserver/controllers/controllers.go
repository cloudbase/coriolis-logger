package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gabriel-samfira/coriolis-logger/datastore/common"
	"github.com/gabriel-samfira/coriolis-logger/logging"
	"github.com/gabriel-samfira/coriolis-logger/params"
	wsWriter "github.com/gabriel-samfira/coriolis-logger/writers/websocket"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/juju/loggo"
	"github.com/pkg/errors"
)

var log = loggo.GetLogger("coriolis.logger.controllers")

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 16384,
}

func NewLogHandler(hub *wsWriter.Hub, datastore common.DataStore) *LogHandlers {
	return &LogHandlers{
		hub:   hub,
		store: datastore,
	}
}

type LogHandlers struct {
	hub   *wsWriter.Hub
	store common.DataStore
}

func getSeverity(severity string) (logging.Severity, error) {
	var ret logging.Severity
	if severity == "" {
		return logging.DefaultSeverityLevel, nil
	}
	cliSeverity, err := strconv.Atoi(severity)
	if err != nil {
		return ret, fmt.Errorf("invalid severity %q", severity)
	}
	if cliSeverity > 7 || cliSeverity < 0 {
		ret = logging.DefaultSeverityLevel
	} else {
		ret = logging.Severity(cliSeverity)
	}
	return ret, nil
}

func (l *LogHandlers) WSHandler(writer http.ResponseWriter, req *http.Request) {
	severityStr := req.URL.Query().Get("severity")
	severity, err := getSeverity(severityStr)
	if err != nil {
		log.Warningf("invalid severity %q. Ignoring", severityStr)
	}
	binName := req.URL.Query().Get("binary_name")

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

func timestampToTime(stamp string) (time.Time, error) {
	if stamp == "" {
		return time.Time{}, nil
	}
	i, err := strconv.ParseInt(stamp, 10, 64)
	if err != nil {
		return time.Time{}, errors.Wrap(err, "converting timestamp")
	}
	tm := time.Unix(i, 0)
	return tm, nil
}

/*
type QueryParams struct {
	Hostname   string
	StartDate  time.Time
	EndDate    time.Time
	BinaryName string
	Severity   int
}

*/

func (l *LogHandlers) DownloadLogHandler(writer http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	severityStr := req.URL.Query().Get("severity")
	severity, err := getSeverity(severityStr)
	if err != nil {
		log.Warningf("invalid severity %q. Ignoring", severityStr)
	}
	if vars["log"] == "" {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(writer, fmt.Sprintf("missing log name"))
	}
	startDateStamp := req.URL.Query().Get("start_date")
	startDate, err := timestampToTime(startDateStamp)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(writer, fmt.Sprintf("invalid start date: %q", startDateStamp))
	}

	endDateStamp := req.URL.Query().Get("end_date")
	endDate, err := timestampToTime(startDateStamp)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(writer, fmt.Sprintf("invalid end date: %q", endDateStamp))
	}

	queryParams := params.QueryParams{
		StartDate:  startDate,
		EndDate:    endDate,
		BinaryName: vars["log"],
		Severity:   int(severity),
	}

	reader := l.store.ResultReader(queryParams)
	for {
		data, err := reader.ReadNext()
		if err != nil {
			if err == io.EOF {
				break
			}
			writer.WriteHeader(http.StatusInternalServerError)
			log.Errorf("error fetching logs: %v", err)
		}
		_, err = writer.Write(data)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			log.Errorf("sending logs: %v", err)
		}
	}
	return
}

func (l *LogHandlers) ListLogsHandler(writer http.ResponseWriter, req *http.Request) {
	logs, err := l.store.List()
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		log.Errorf("error listing logs: %v", err)
	}
	js, err := json.Marshal(logs)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		log.Errorf("error listing logs: %v", err)
	}
	fmt.Fprintf(writer, string(js))
}
