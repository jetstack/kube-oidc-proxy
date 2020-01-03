// Copyright Jetstack Ltd. See LICENSE for details.
package util

import (
	"net/http"
	"strings"
	"time"

	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
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

// fakeJWT generates a valid JWT using the passed input parameters which is
// signed by a generated key. This is useful for checking the status of a
// signer.
func FakeJWT(issuerURL string, apiAudiences []string) (string, error) {
	key := []byte("secret")

	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.HS256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		return "", err
	}

	cl := jwt.Claims{
		Subject:   "fake",
		Issuer:    issuerURL,
		NotBefore: jwt.NewNumericDate(time.Date(2016, 1, 1, 0, 0, 0, 0, time.UTC)),
		Audience:  jwt.Audience(apiAudiences),
	}

	return jwt.Signed(sig).Claims(cl).CompactSerialize()
}
