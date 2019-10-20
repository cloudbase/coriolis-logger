package auth

import (
	"fmt"
	"net/http"
)

var (
	AuthenticationDisabledErr = fmt.Errorf("authentication disabled")
)

type handler struct {
	auth    Authenticator
	handler http.Handler
}

func (h *handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx, err := h.auth.Authenticate(req)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to authenticate: %v", err)
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(errMsg))
		log.Errorf(errMsg)
		return
	}
	h.handler.ServeHTTP(w, req.WithContext(ctx))
}
