// Copyright Jetstack Ltd. See LICENSE for details.
package proxy

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	//"k8s.io/klog"
)

// Validate service account token
func (p *Proxy) validateServiceAccountToken(req *http.Request) error {
	token, err := parseTokenFromHeader(req)
	if err != nil {
		return err
	}

	_, ok, err := p.saAuther.AuthenticateToken(req.Context(), token)
	if err != nil {
		return fmt.Errorf("failed to authenticate possible service account request token: %s",
			err)
	}

	if !ok {
		return errors.New("token also failed service account authentication")
	}

	return nil
}

// Return just the token from the header of the request, without 'bearer'.
func parseTokenFromHeader(req *http.Request) (string, error) {
	auth := strings.TrimSpace(req.Header.Get("Authorization"))
	if auth == "" {
		return "", errTokenParse
	}

	parts := strings.Split(auth, " ")
	if len(parts) < 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", errTokenParse
	}

	token := parts[1]

	// Empty bearer tokens aren't valid
	if len(token) == 0 {
		return "", errTokenParse
	}

	return token, nil
}
