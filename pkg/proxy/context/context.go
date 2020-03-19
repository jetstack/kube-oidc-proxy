// Copyright Jetstack Ltd. See LICENSE for details.
package context

import (
	"net/http"

	"github.com/sebest/xff"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/transport"
)

type key int

const (
	// noImpersonationKey is the context key for whether to use impersonation.
	noImpersonationKey key = iota

	// impersonationConfigKey is the context key for the impersonation config.
	impersonationConfigKey

	// bearerTokenKey is the context key for the bearer token.
	bearerTokenKey

	// bearerTokenKey is the context key for the client address.
	clientAddressKey
)

// WithNoImpersonation returns a copy of the request in which the noImpersonation context value is set.
func WithNoImpersonation(req *http.Request) *http.Request {
	return req.WithContext(request.WithValue(req.Context(), noImpersonationKey, true))
}

// NoImpersonation returns whether the noImpersonation context key has been set
func NoImpersonation(req *http.Request) bool {
	noImp, _ := req.Context().Value(noImpersonationKey).(bool)
	return noImp
}

// WithImpersonationConfig returns a copy of parent in which contains the impersonation configuration.
func WithImpersonationConfig(req *http.Request, conf *transport.ImpersonationConfig) *http.Request {
	return req.WithContext(request.WithValue(req.Context(), impersonationConfigKey, conf))
}

// ImpersonationConfig returns the impersonation configuration held in the context if existing.
func ImpersonationConfig(req *http.Request) *transport.ImpersonationConfig {
	conf, _ := req.Context().Value(impersonationConfigKey).(*transport.ImpersonationConfig)
	return conf
}

// WithBearerToken will add the bearer token to the request context from an http.Header to the request context.
func WithBearerToken(req *http.Request, header http.Header) *http.Request {
	return req.WithContext(request.WithValue(req.Context(), bearerTokenKey, header.Get("Authorization")))
}

// BearerToken will return the bearer token stored in the request context.
func BearerToken(req *http.Request) string {
	token, _ := req.Context().Value(bearerTokenKey).(string)
	return token
}

// RemoteAddress will attempt to return the source client address if available
// in the request context. If it is not, it will be gathered from the request
// and entered into the context.
func RemoteAddr(req *http.Request) (*http.Request, string) {
	ctx := req.Context()

	clientAddress, ok := ctx.Value(clientAddressKey).(string)
	if !ok {
		clientAddress = xff.GetRemoteAddr(req)
		req = req.WithContext(request.WithValue(ctx, clientAddressKey, clientAddress))
	}

	return req, clientAddress
}
