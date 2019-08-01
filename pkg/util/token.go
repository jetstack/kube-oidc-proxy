// Copyright Jetstack Ltd. See LICENSE for details.
package util

import (
	"net/http"
	"strings"
)

// Return just the token from the header of the request, without 'bearer'.
func ParseTokenFromRequest(req *http.Request) (string, bool) {
	if req == nil || req.Header == nil {
		return "", false
	}

	auth := strings.TrimSpace(req.Header.Get("Authorization"))
	if auth == "" {
		return "", false
	}

	parts := strings.Split(auth, " ")
	if len(parts) < 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", false
	}

	token := parts[1]

	// Empty bearer tokens aren't valid
	if len(token) == 0 {
		return "", false
	}

	return token, true
}
