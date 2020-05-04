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

package auth

import (
	"fmt"
	"net/http"
	"time"

	"coriolis-logger/config"

	"github.com/databus23/keystone"
	"github.com/juju/loggo"
	"github.com/pkg/errors"
)

const (
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
		cfg:  cfg,
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
