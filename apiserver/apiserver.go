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

package apiserver

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gabriel-samfira/coriolis-logger/apiserver/controllers"
	"github.com/gabriel-samfira/coriolis-logger/apiserver/routers"
	"github.com/gabriel-samfira/coriolis-logger/config"
	"github.com/gabriel-samfira/coriolis-logger/datastore/common"
	wsWriter "github.com/gabriel-samfira/coriolis-logger/writers/websocket"
	"github.com/pkg/errors"
)

type APIServer struct {
	listener net.Listener
	srv      *http.Server
}

func (h *APIServer) Start() error {
	go func() {
		if err := h.srv.Serve(h.listener); err != nil {
			log.Fatal(err)
		}
	}()
	return nil
}

func (h *APIServer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown web server: %q", err)
	}

	return nil
}

func GetAPIServer(cfg config.APIServer, hub *wsWriter.Hub, datastore common.DataStore) (*APIServer, error) {
	logHandler := controllers.NewLogHandler(hub, datastore)
	router, err := routers.GetRouter(cfg, logHandler)
	if err != nil {
		return nil, errors.Wrap(err, "getting router")
	}
	srv := &http.Server{
		Handler: router,
	}
	if cfg.UseTLS {
		tlsCfg, err := cfg.TLSConfig.TLSConfig()
		if err != nil {
			return nil, errors.Wrap(err, "getting TLS config")
		}
		srv.TLSConfig = tlsCfg
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Bind, cfg.Port))
	if err != nil {
		return nil, err
	}
	return &APIServer{
		srv:      srv,
		listener: listener,
	}, nil
}
