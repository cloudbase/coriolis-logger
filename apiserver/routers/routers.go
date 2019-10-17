package routers

import (
	"net/http"
	"os"

	"github.com/gabriel-samfira/coriolis-logger/apiserver/controllers"
	gorillaHandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func GetRouter(han *controllers.LogHandlers) *mux.Router {
	router := mux.NewRouter()
	apiRouter := router.PathPrefix("/api/v1").Subrouter()

	// I was too lazy to look through documentation on how to treat the trailing slash properly.
	// Pull requests welcome
	apiRouter.Handle("/ws", gorillaHandlers.LoggingHandler(os.Stdout, http.HandlerFunc(han.WSHandler))).Methods("GET")
	apiRouter.Handle("/ws/", gorillaHandlers.LoggingHandler(os.Stdout, http.HandlerFunc(han.WSHandler))).Methods("GET")

	return router
}
