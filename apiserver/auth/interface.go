// Copyright 2019 Cloudbase Solutions SRL

package auth

import (
	"context"
	"net/http"
)

type Authenticator interface {
	Authenticate(req *http.Request) (context.Context, error)
}

type MiddlewareWrapper interface {
	Handler(h http.Handler) http.Handler
}
