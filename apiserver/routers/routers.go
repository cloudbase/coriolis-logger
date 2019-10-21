package routers

// Copyright 2019 Cloudbase Solutions SRL

import (
	"net/http"
	"os"

	"github.com/gabriel-samfira/coriolis-logger/apiserver/auth"
	"github.com/gabriel-samfira/coriolis-logger/apiserver/controllers"
	"github.com/gabriel-samfira/coriolis-logger/config"
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
