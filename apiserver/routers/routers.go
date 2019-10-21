package routers

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

import (
	"net/http"
	"os"

	"coriolis-logger/apiserver/auth"
	"coriolis-logger/apiserver/controllers"
	"coriolis-logger/config"
	gorillaHandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

func GetRouter(cfg config.APIServer, han *controllers.LogHandlers) (*mux.Router, error) {
	router := mux.NewRouter()
	apiRouter := router.PathPrefix("/api/v1").Subrouter()
	authMiddleware, err := auth.GetAuthMiddleware(cfg)
	if err != nil {
		if err != auth.AuthenticationDisabledErr {
			return nil, errors.Wrap(err, "getting auth middleware")
		}
	} else {
		apiRouter.Use(authMiddleware.Handler)
	}

	apiRouter.Handle("/{ws:ws\\/?}", gorillaHandlers.LoggingHandler(os.Stdout, http.HandlerFunc(han.WSHandler))).Methods("GET")
	apiRouter.Handle("/{logs:logs\\/?}", gorillaHandlers.LoggingHandler(os.Stdout, http.HandlerFunc(han.ListLogsHandler))).Methods("GET")
	apiRouter.Handle("/logs/{log}", gorillaHandlers.LoggingHandler(os.Stdout, http.HandlerFunc(han.DownloadLogHandler))).Methods("GET")
	apiRouter.Handle("/logs/{log}/", gorillaHandlers.LoggingHandler(os.Stdout, http.HandlerFunc(han.DownloadLogHandler))).Methods("GET")

	return router, nil
}
