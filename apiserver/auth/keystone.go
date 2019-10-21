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
	"context"
	"fmt"
	"net/http"

	"github.com/databus23/keystone"
	"github.com/pkg/errors"
)

type keystoneAuth struct {
	auth *keystone.Auth
}

func (k keystoneAuth) Authenticate(req *http.Request) (context.Context, error) {
	authToken := req.Header.Get("X-Auth-Token")
	if authToken == "" {
		return nil, fmt.Errorf("missing token in headers")
	}

	keystoneContext, err := k.auth.Validate(authToken)
	if err != nil {
		return nil, errors.Wrap(err, "authenticating token")
	}

	var isAdmin bool
	for _, val := range keystoneContext.Roles {
		if val.Name == "admin" {
			isAdmin = true
			break
		}
	}
	authDetails := AuthDetails{
		UserID:    keystoneContext.User.ID,
		IsAdmin:   isAdmin,
		ExpiresAt: keystoneContext.ExpiresAt,
	}

	ctx := req.Context()

	return context.WithValue(ctx, AuthDetailsKey, authDetails), nil
}
