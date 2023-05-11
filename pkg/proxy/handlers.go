// Copyright Jetstack Ltd. See LICENSE for details.
package proxy

import (
	"errors"
	"net/http"
	"strings"
	"time"

	authuser "k8s.io/apiserver/pkg/authentication/user"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/transport"
	"k8s.io/klog"

	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/audit"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/context"
)

const (
	UserHeaderClientIPKey = "Remote-Client-IP"
)

var (
	errUnauthorized          = errors.New("Unauthorized")
	errImpersonateHeader     = errors.New("Impersonate-User in header")
	errNoName                = errors.New("No name in OIDC info")
	errNoImpersonationConfig = errors.New("No impersonation configuration in context")

	// http headers are case-insensitive
	impersonateUserHeader  = strings.ToLower(transport.ImpersonateUserHeader)
	impersonateGroupHeader = strings.ToLower(transport.ImpersonateGroupHeader)
	impersonateExtraHeader = strings.ToLower(transport.ImpersonateUserExtraHeaderPrefix)
)

func (p *Proxy) withHandlers(handler http.Handler) http.Handler {
	// Set up proxy handlers
	handler = p.withClientTimestamp(handler)
	handler = p.auditor.WithRequest(handler)
	handler = p.withImpersonateRequest(handler)
	handler = p.withAuthenticateRequest(handler)

	// Add the auditor backend as a shutdown hook
	p.hooks.AddPreShutdownHook("AuditBackend", p.auditor.Shutdown)

	return handler
}

// withAuthenticateRequest adds the proxy authentication handler to a chain.
func (p *Proxy) withAuthenticateRequest(handler http.Handler) http.Handler {
	tokenReviewHandler := p.withTokenReview(handler)

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		req, remoteAddr := context.RemoteAddr(req)

		// Auth request and handle unauthed
		info, ok, err := p.oidcRequestAuther.AuthenticateRequest(req)
		if err != nil {
			// Since we have failed OIDC auth, we will try a token review, if enabled.
			p.metrics.IncrementOIDCAuthCount(false, remoteAddr, "")
			tokenReviewHandler.ServeHTTP(rw, req)
			return
		}

		// Failed authorization
		if !ok {
			p.metrics.IncrementOIDCAuthCount(false, remoteAddr, "")
			p.handleError(rw, req, errUnauthorized)
			return
		}

		klog.V(4).Infof("authenticated request: %s", remoteAddr)

		// Add the user info to the request context
		req = req.WithContext(genericapirequest.WithUser(req.Context(), info.User))
		p.metrics.IncrementOIDCAuthCount(true, remoteAddr, info.User.GetName())
		handler.ServeHTTP(rw, req)
	})
}

// withTokenReview will attempt a token review on the incoming request, if
// enabled.
func (p *Proxy) withTokenReview(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// If token review is not enabled then error.
		if !p.config.TokenReview {
			p.handleError(rw, req, errUnauthorized)
			return
		}

		// Attempt to passthrough request if valid token
		if !p.reviewToken(rw, req) {
			// Token review failed so error
			p.handleError(rw, req, errUnauthorized)
			return
		}

		// Set no impersonation headers and re-add removed headers.
		req = context.WithNoImpersonation(req)

		handler.ServeHTTP(rw, req)
	})
}

// withImpersonateRequest adds the impersonation request handler to the chain.
func (p *Proxy) withImpersonateRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// If no impersonation has already been set, return early
		if context.NoImpersonation(req) {
			handler.ServeHTTP(rw, req)
			return
		}

		req, remoteAddr := context.RemoteAddr(req)

		// If we have disabled impersonation we can forward the request right away
		if p.config.DisableImpersonation {
			klog.V(2).Infof("passing on request with no impersonation: %s", remoteAddr)
			// Indicate we need to not use impersonation.
			req = context.WithNoImpersonation(req)
			handler.ServeHTTP(rw, req)
			return
		}

		if p.hasImpersonation(req.Header) {
			p.handleError(rw, req, errImpersonateHeader)
			return
		}

		user, ok := genericapirequest.UserFrom(req.Context())
		// No name available so reject request
		if !ok || len(user.GetName()) == 0 {
			p.handleError(rw, req, errNoName)
			return
		}

		// Ensure group contains allauthenticated builtin
		allAuthFound := false
		groups := user.GetGroups()
		for _, elem := range groups {
			if elem == authuser.AllAuthenticated {
				allAuthFound = true
				break
			}
		}
		if !allAuthFound {
			groups = append(groups, authuser.AllAuthenticated)
		}

		extra := user.GetExtra()

		if extra == nil {
			extra = make(map[string][]string)
		}

		// If client IP user extra header option set then append the remote client
		// address.
		if p.config.ExtraUserHeadersClientIPEnabled {
			klog.V(6).Infof("adding impersonate extra user header %s: %s (%s)",
				UserHeaderClientIPKey, remoteAddr, remoteAddr)

			extra[UserHeaderClientIPKey] = append(extra[UserHeaderClientIPKey], remoteAddr)
		}

		// Add custom extra user headers to impersonation request.
		for k, vs := range p.config.ExtraUserHeaders {
			for _, v := range vs {
				klog.V(6).Infof("adding impersonate extra user header %s: %s (%s)",
					k, v, remoteAddr)

				extra[k] = append(extra[k], v)
			}
		}

		conf := &transport.ImpersonationConfig{
			UserName: user.GetName(),
			Groups:   groups,
			Extra:    extra,
		}

		// Add the impersonation configuration to the context.
		req = context.WithImpersonationConfig(req, conf)
		handler.ServeHTTP(rw, req)
	})
}

// withClientTimestamp adds the current timestamp for the client request to the
// request context.
func (p *Proxy) withClientTimestamp(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		req = context.WithClientRequestTimestamp(req)
		handler.ServeHTTP(rw, req)
	})
}

// newErrorHandler returns a handler failed requests.
func (p *Proxy) newErrorHandler() func(rw http.ResponseWriter, req *http.Request, err error) {

	// Setup unauthed handler so that it is passed through the audit
	unauthedHandler := audit.NewUnauthenticatedHandler(p.auditor, func(rw http.ResponseWriter, req *http.Request) {
		_, remoteAddr := context.RemoteAddr(req)
		klog.V(2).Infof("unauthenticated user request %s", remoteAddr)
		http.Error(rw, "Unauthorized", http.StatusUnauthorized)
	})

	return func(rw http.ResponseWriter, req *http.Request, err error) {
		var statusCode int
		req, remoteAddr := context.RemoteAddr(req)

		// Update client duration metrics from error
		defer func() {
			clientDuration := context.ClientRequestTimestamp(req)
			p.metrics.ObserveClient(statusCode, req.URL.Path, remoteAddr, time.Since(clientDuration))
		}()

		if err == nil {
			klog.Error("error was called with no error")
			http.Error(rw, "", http.StatusInternalServerError)
			return
		}

		switch err {

		// Failed auth
		case errUnauthorized:
			// If Unauthorized then error and report to audit
			statusCode = http.StatusUnauthorized
			unauthedHandler.ServeHTTP(rw, req)
			return

			// User request with impersonation
		case errImpersonateHeader:
			statusCode = http.StatusForbidden
			klog.V(2).Infof("impersonation user request %s", remoteAddr)
			http.Error(rw, "Impersonation requests are disabled when using kube-oidc-proxy", statusCode)
			return

			// No name given or available in oidc request
		case errNoName:
			statusCode = http.StatusForbidden
			klog.V(2).Infof("no name available in oidc info %s", remoteAddr)
			http.Error(rw, "Username claim not available in OIDC Issuer response", statusCode)
			return

			// No impersonation configuration found in context
		case errNoImpersonationConfig:
			statusCode = http.StatusInternalServerError
			klog.Errorf("if you are seeing this, there is likely a bug in the proxy (%s): %s", remoteAddr, err)
			http.Error(rw, "", statusCode)
			return

			// Server or unknown error
		default:
			statusCode = http.StatusInternalServerError
			klog.Errorf("unknown error (%s): %s", remoteAddr, err)
			http.Error(rw, "", statusCode)
		}
	}
}

func (p *Proxy) hasImpersonation(header http.Header) bool {
	for h := range header {
		if strings.ToLower(h) == impersonateUserHeader ||
			strings.ToLower(h) == impersonateGroupHeader ||
			strings.HasPrefix(strings.ToLower(h), impersonateExtraHeader) {

			return true
		}
	}

	return false
}
