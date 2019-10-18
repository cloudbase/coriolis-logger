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

	apiRouter.Handle("/{ws:ws\\/?}", gorillaHandlers.LoggingHandler(os.Stdout, http.HandlerFunc(han.WSHandler))).Methods("GET")

	return router
}
