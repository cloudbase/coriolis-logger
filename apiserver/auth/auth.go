// Copyright 2019 Cloudbase Solutions SRL

package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/databus23/keystone"
	"github.com/gabriel-samfira/coriolis-logger/config"
	"github.com/juju/loggo"
	"github.com/pkg/errors"
)

const (
	adminRoleName  = "admin"
	AuthDetailsKey = "auth_details"
)

var log = loggo.GetLogger("coriolis.logger.apiserver.auth")

type middlewareWrapper struct {
	a Authenticator
}

func (m *middlewareWrapper) Handler(h http.Handler) http.Handler {
	return &handler{
		auth:    m.a,
		handler: h,
	}
}

// AuthDetails represents information about an authenticated user
// At the moment it only stores the user ID and a boolean indicating
// whether or not the user is an admin. This can be later extended
// to hold more info
type AuthDetails struct {
	UserID    string
	IsAdmin   bool
	ExpiresAt time.Time
}

func getKeystoneAuthenticator(cfg *config.KeystoneAuth) (Authenticator, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(err, "validating keystone config")
	}
	auth := keystone.New(cfg.AuthURI)
	return keystoneAuth{
		auth: auth,
	}, nil
}

func GetAuthMiddleware(cfg config.APIServer) (MiddlewareWrapper, error) {
	switch cfg.AuthMiddleware {
	case config.AuthenticationKeystone:
		authenticator, err := getKeystoneAuthenticator(cfg.KeystoneAuth)
		if err != nil {
			return nil, errors.Wrap(err, "getting keystone authenticator")
		}
		return &middlewareWrapper{
			a: authenticator,
		}, nil
	case config.AuthenticationNone:
		return nil, AuthenticationDisabledErr
	default:
		return nil, fmt.Errorf("could not find authentication middleware %q", cfg.AuthMiddleware)
	}
}
