// Copyright Jetstack Ltd. See LICENSE for details.
package context

import (
	"context"
	"net/http"

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
)

// WithNoImpersonation returns a copy of parent in which the noImpersonation value is set.
func WithNoImpersonation(parent context.Context) context.Context {
	return request.WithValue(parent, noImpersonationKey, true)
}

// NoImpersonation returns whether the noImpersonation key has been set
func NoImpersonation(ctx context.Context) bool {
	noImp, _ := ctx.Value(noImpersonationKey).(bool)
	return noImp
}

// WithImpersonationConfig returns a copy of parent in which contains the impersonation configuration.
func WithImpersonationConfig(parent context.Context, conf *transport.ImpersonationConfig) context.Context {
	return request.WithValue(parent, impersonationConfigKey, conf)
}

// ImpersonationConfig returns the impersonation configuration held in the context if existing.
func ImpersonationConfig(ctx context.Context) *transport.ImpersonationConfig {
	conf, _ := ctx.Value(impersonationConfigKey).(*transport.ImpersonationConfig)
	return conf
}

// WithBearerToken will add the bearer token from an http.Header to the context.
func WithBearerToken(parent context.Context, header http.Header) context.Context {
	return request.WithValue(parent, bearerTokenKey, header.Get("Authorization"))
}

// BearerToken will return the bearer token stored in the context.
func BearerToken(ctx context.Context) string {
	token, _ := ctx.Value(bearerTokenKey).(string)
	return token
}
