// Copyright 2019 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"coriolis-logger/apiserver/auth"
	"coriolis-logger/config"
	"coriolis-logger/datastore/common"
	"coriolis-logger/logging"
	"coriolis-logger/params"
	wsWriter "coriolis-logger/writers/websocket"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/juju/loggo"
	"github.com/pkg/errors"
)

var log = loggo.GetLogger("coriolis.logger.controllers")

func canAccess(ctx context.Context) bool {
	details := ctx.Value(auth.AuthDetailsKey)
	if details == nil {
		return false
	}
	authDetails := details.(auth.AuthDetails)
	// TODO (gsamfira): allow policy based access
	return authDetails.IsAdmin
}

func NewLogHandler(hub *wsWriter.Hub, datastore common.DataStore, cfg config.APIServer) *LogHandlers {
	han := &LogHandlers{
		hub:   hub,
		store: datastore,
		cfg:   cfg,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 16384,
		},
	}

	corsChecker := han.getCORSChecker()
	han.upgrader.CheckOrigin = corsChecker
	return han
}

type LogHandlers struct {
	hub      *wsWriter.Hub
	store    common.DataStore
	cfg      config.APIServer
	upgrader websocket.Upgrader
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

func (l *LogHandlers) getCORSChecker() func(r *http.Request) bool {
	if l.cfg.CORSOrigins == nil || len(l.cfg.CORSOrigins) == 0 {
		return nil
	}

	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		for _, val := range l.cfg.CORSOrigins {
			if val == "*" || val == origin {
				return true
			}
		}
		return false
	}
}

func (l *LogHandlers) WSHandler(writer http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if !canAccess(ctx) {
		writer.WriteHeader(http.StatusForbidden)
		writer.Write([]byte("you need admin level access to view logs"))
		return
	}
	severityStr := req.URL.Query().Get("severity")
	severity, err := getSeverity(severityStr)
	if err != nil {
		log.Warningf("invalid severity %q. Ignoring", severityStr)
	}
	binName := req.URL.Query().Get("app_name")

	conn, err := l.upgrader.Upgrade(writer, req, nil)
	if err != nil {
		log.Errorf("error upgrading to websockets: %v", err)
		return
	}

	opts := wsWriter.ClientFilterOptions{
		Severity: &severity,
		AppName:  &binName,
	}
	// TODO (gsamfira): Handle ExpiresAt. Right now, if a client uses
	// a valid token to authenticate, and keeps the websocket connection
	// open, it will allow that client to stream logs via websockets
	// until the connection is broken. We need to forcefully disconnect
	// the client once the token expires.
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

// downloadAsFile prepares a log for download by creating a temporary file
// to which it dumps the log, then serves it as a plain file to the client.
// This is done because some browsers like Safari have issues with
// chunked downloads. This is a workaround that should be removed at a later
// time.
func (l *LogHandlers) downloadAsFile(reader common.Reader, writer http.ResponseWriter, logName string) {
	tmpfile, err := ioutil.TempFile("", "coriolis-logger")
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		log.Errorf("error creating temp file: %v", err)
		return
	}

	defer func() {
		tmpfile.Close()
		os.Remove(tmpfile.Name())
	}()

	for {
		data, err := reader.ReadNext()
		if err != nil {
			if err == io.EOF {
				break
			}
			writer.WriteHeader(http.StatusInternalServerError)
			log.Errorf("error reading log: %v", err)
			return
		}
		_, err = tmpfile.Write(data)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			log.Errorf("error writing to temp file: %v", err)
			return
		}
	}

	logStat, err := tmpfile.Stat()
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		log.Errorf("error getting log info: %v", err)
		return
	}

	if _, err := tmpfile.Seek(0, 0); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		log.Errorf("error seeking log: %v", err)
		return
	}

	size := strconv.FormatInt(logStat.Size(), 10)
	writer.Header().Set("Content-Disposition", "attachment; filename="+logName)
	writer.Header().Set("Content-Type", "text/plain")
	writer.Header().Set("Content-Length", size)

	if _, err := io.Copy(writer, tmpfile); err != nil {
		log.Errorf("error sending file: %v", err)
		return
	}
	return
}

func (l *LogHandlers) downloadAsChuks(reader common.Reader, writer http.ResponseWriter, logName string) {
	data, err := reader.ReadNext()
	if err != nil {
		if err != io.EOF {
			writer.WriteHeader(http.StatusInternalServerError)
			log.Errorf("error fetching logs: %v", err)
			return
		}
	}
	writer.Header().Set("Content-Disposition", "attachment; filename="+logName)
	writer.Header().Set("Content-Type", "text/plain")

	_, err = writer.Write(data)
	if err != nil {
		log.Errorf("sending logs: %v", err)
		return
	}

	for {
		data, err := reader.ReadNext()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Errorf("error fetching logs: %v", err)
			return
		}
		_, err = writer.Write(data)
		if err != nil {
			log.Errorf("sending logs: %v", err)
			return
		}
	}
	return
}

func (l *LogHandlers) DownloadLogHandler(writer http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if !canAccess(ctx) {
		writer.WriteHeader(http.StatusForbidden)
		writer.Write([]byte("you need admin level access to view logs"))
		return
	}
	disableChunked := req.URL.Query().Get("disable_chunked")
	disableChunkedAsBool, _ := strconv.ParseBool(disableChunked)

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
	endDate, err := timestampToTime(endDateStamp)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(writer, fmt.Sprintf("invalid end date: %q", endDateStamp))
	}

	queryParams := params.QueryParams{
		StartDate: startDate,
		EndDate:   endDate,
		AppName:   vars["log"],
		Severity:  int(severity),
	}

	reader := l.store.ResultReader(queryParams)
	if disableChunkedAsBool {
		l.downloadAsFile(reader, writer, vars["log"])
		return
	}
	l.downloadAsChuks(reader, writer, vars["log"])
	return
}

func (l *LogHandlers) ListLogsHandler(writer http.ResponseWriter, req *http.Request) {
	logs, err := l.store.List()
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		log.Errorf("error listing logs: %v", err)
	}
	ret := map[string][]map[string]string{
		"logs": logs,
	}
	js, err := json.Marshal(ret)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		log.Errorf("error listing logs: %v", err)
	}
	fmt.Fprintf(writer, string(js))
}
