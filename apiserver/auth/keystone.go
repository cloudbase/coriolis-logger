// Copyright 2019 Cloudbase Solutions SRL

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
