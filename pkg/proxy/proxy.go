// Copyright Jetstack Ltd. See LICENSE for details.
package proxy

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	authuser "k8s.io/apiserver/pkg/authentication/user"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/plugin/pkg/authenticator/token/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"k8s.io/klog"

	"github.com/jetstack/kube-oidc-proxy/cmd/app/options"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/context"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/tokenreview"
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

type Config struct {
	DisableImpersonation bool
	TokenReview          bool

	FlushInterval time.Duration

	ExtraUserHeaders                map[string][]string
	ExtraUserHeadersClientIPEnabled bool
}

type errorHandlerFn func(http.ResponseWriter, *http.Request, error)

type Proxy struct {
	oidcRequestAuther *bearertoken.Authenticator
	tokenAuther       authenticator.Token
	tokenReviewer     *tokenreview.TokenReview
	secureServingInfo *server.SecureServingInfo

	restConfig            *rest.Config
	clientTransport       http.RoundTripper
	noAuthClientTransport http.RoundTripper

	config *Config

	handleError errorHandlerFn
}

func New(restConfig *rest.Config, oidcOptions *options.OIDCAuthenticationOptions,
	tokenReviewer *tokenreview.TokenReview, ssinfo *server.SecureServingInfo,
	config *Config) (*Proxy, error) {

	// generate tokenAuther from oidc config
	tokenAuther, err := oidc.New(oidc.Options{
		APIAudiences:         oidcOptions.APIAudiences,
		CAFile:               oidcOptions.CAFile,
		ClientID:             oidcOptions.ClientID,
		GroupsClaim:          oidcOptions.GroupsClaim,
		GroupsPrefix:         oidcOptions.GroupsPrefix,
		IssuerURL:            oidcOptions.IssuerURL,
		RequiredClaims:       oidcOptions.RequiredClaims,
		SupportedSigningAlgs: oidcOptions.SigningAlgs,
		UsernameClaim:        oidcOptions.UsernameClaim,
		UsernamePrefix:       oidcOptions.UsernamePrefix,
	})
	if err != nil {
		return nil, err
	}

	return &Proxy{
		restConfig:        restConfig,
		tokenReviewer:     tokenReviewer,
		secureServingInfo: ssinfo,
		config:            config,
		oidcRequestAuther: bearertoken.New(tokenAuther),
		tokenAuther:       tokenAuther,
	}, nil
}

func (p *Proxy) Run(stopCh <-chan struct{}) (<-chan struct{}, error) {
	// standard round tripper for proxy to API Server
	clientRT, err := p.roundTripperForRestConfig(p.restConfig)
	if err != nil {
		return nil, err
	}
	p.clientTransport = clientRT

	// No auth round tripper for no impersonation
	if p.config.DisableImpersonation || p.config.TokenReview {
		noAuthClientRT, err := p.roundTripperForRestConfig(&rest.Config{
			APIPath: p.restConfig.APIPath,
			Host:    p.restConfig.Host,
			Timeout: p.restConfig.Timeout,
			TLSClientConfig: rest.TLSClientConfig{
				CAFile: p.restConfig.CAFile,
				CAData: p.restConfig.CAData,
			},
		})
		if err != nil {
			return nil, err
		}

		p.noAuthClientTransport = noAuthClientRT
	}

	// get API server url
	url, err := url.Parse(p.restConfig.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %s", err)
	}

	p.handleError = p.newErrorHandler()

	// Set up proxy handler using proxy
	proxyHandler := httputil.NewSingleHostReverseProxy(url)
	proxyHandler.Transport = p
	proxyHandler.ErrorHandler = p.handleError
	proxyHandler.FlushInterval = p.config.FlushInterval

	waitCh, err := p.serve(proxyHandler, stopCh)
	if err != nil {
		return nil, err
	}

	return waitCh, nil
}

func (p *Proxy) serve(handler http.Handler, stopCh <-chan struct{}) (<-chan struct{}, error) {
	// Setup proxy handlers
	handler = p.withHandlers(handler)

	// securely serve using serving config
	waitCh, err := p.secureServingInfo.Serve(handler, time.Second*60, stopCh)
	if err != nil {
		return nil, err
	}

	return waitCh, nil
}

func (p *Proxy) withHandlers(handler http.Handler) http.Handler {
	// Set up proxy handlers
	handler = p.withImpersonateRequest(handler)
	handler = p.withAuthenticateRequest(handler)
	return handler
}

// RoundTrip is called last and is used to manipulate the forwarded request using context.
func (p *Proxy) RoundTrip(req *http.Request) (*http.Response, error) {
	// Here we have successfully authenticated so now need to determine whether
	// we need use impersonation or not.

	// If no impersonation then we return here without setting impersonation
	// header but re-introduce the token we removed.
	if context.NoImpersonation(req.Context()) {
		token := context.BearerToken(req.Context())
		req.Header.Add("Authorization", token)
		return p.noAuthClientTransport.RoundTrip(req)
	}

	// Get the impersonation headers from the context.
	conf := context.ImpersonationConfig(req.Context())
	if conf == nil {
		return nil, errNoImpersonationConfig
	}

	// Set up impersonation request.
	rt := transport.NewImpersonatingRoundTripper(*conf, p.clientTransport)

	// Push request through round trippers to the API server.
	return rt.RoundTrip(req)
}

// withAuthenticateRequest adds the proxy authentication handler to a chain.
func (p *Proxy) withAuthenticateRequest(handler http.Handler) http.Handler {
	tokenReviewHandler := p.withTokenReview(handler)

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Auth request and handle unauthed
		info, ok, err := p.oidcRequestAuther.AuthenticateRequest(req)
		if err != nil {
			// Since we have failed OIDC auth, we will try a token review, if enabled.
			tokenReviewHandler.ServeHTTP(rw, req)
			return
		}

		// Failed authorization
		if !ok {
			p.handleError(rw, req, errUnauthorized)
			return
		}

		klog.V(4).Infof("authenticated request: %s", req.RemoteAddr)

		// Add the user info to the request context
		req = req.WithContext(genericapirequest.WithUser(req.Context(), info.User))
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
		req = req.WithContext(context.WithNoImpersonation(req.Context()))

		handler.ServeHTTP(rw, req)
	})
}

// withImpersonateRequest adds the impersonation request handler to the chain.
func (p *Proxy) withImpersonateRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// If no impersonation has already been set, return early
		if context.NoImpersonation(req.Context()) {
			handler.ServeHTTP(rw, req)
			return
		}

		// If we have disabled impersonation we can forward the request right away
		if p.config.DisableImpersonation {
			klog.V(2).Infof("passing on request with no impersonation: %s", req.RemoteAddr)
			// Indicate we need to not use impersonation.
			req = req.WithContext(context.WithNoImpersonation(req.Context()))
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
				UserHeaderClientIPKey, req.RemoteAddr, req.RemoteAddr)

			extra[UserHeaderClientIPKey] = append(extra[UserHeaderClientIPKey], req.RemoteAddr)
		}

		// Add custom extra user headers to impersonation request.
		for k, vs := range p.config.ExtraUserHeaders {
			for _, v := range vs {
				klog.V(6).Infof("adding impersonate extra user header %s: %s (%s)",
					k, v, req.RemoteAddr)

				extra[k] = append(extra[k], v)
			}
		}

		conf := &transport.ImpersonationConfig{
			UserName: user.GetName(),
			Groups:   groups,
			Extra:    extra,
		}

		// Add the impersonation configuration to the context.
		req = req.WithContext(context.WithImpersonationConfig(req.Context(), conf))
		handler.ServeHTTP(rw, req)
	})
}

func (p *Proxy) reviewToken(rw http.ResponseWriter, req *http.Request) bool {
	klog.V(4).Infof("attempting to validate a token in request using TokenReview endpoint(%s)",
		req.RemoteAddr)

	ok, err := p.tokenReviewer.Review(req)
	if err != nil {
		klog.Errorf("unable to authenticate the request via TokenReview due to an error (%s): %s",
			req.RemoteAddr, err)
		return false
	}

	if !ok {
		klog.V(4).Infof("passing request with valid token through (%s)",
			req.RemoteAddr)

		return false
	}

	// No error and ok so passthrough the request
	return true
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

// newErrorHandler returns a handler failed requests.
func (p *Proxy) newErrorHandler() func(rw http.ResponseWriter, r *http.Request, err error) {
	return func(rw http.ResponseWriter, r *http.Request, err error) {
		if err == nil {
			klog.Error("error was called with no error")
			http.Error(rw, "", http.StatusInternalServerError)
			return
		}

		switch err {

		// Failed auth
		case errUnauthorized:
			klog.V(2).Infof("unauthenticated user request %s", r.RemoteAddr)
			http.Error(rw, "Unauthorized", http.StatusUnauthorized)
			return

			// User request with impersonation
		case errImpersonateHeader:
			klog.V(2).Infof("impersonation user request %s", r.RemoteAddr)
			http.Error(rw, "Impersonation requests are disabled when using kube-oidc-proxy", http.StatusForbidden)
			return

			// No name given or available in oidc request
		case errNoName:
			klog.V(2).Infof("no name available in oidc info %s", r.RemoteAddr)
			http.Error(rw, "Username claim not available in OIDC Issuer response", http.StatusForbidden)
			return

			// No impersonation configuration found in context
		case errNoImpersonationConfig:
			klog.Errorf("if you are seeing this, there is likely a bug in the proxy (%s): %s", r.RemoteAddr, err)
			http.Error(rw, "", http.StatusInternalServerError)
			return

			// Server or unknown error
		default:
			klog.Errorf("unknown error (%s): %s", r.RemoteAddr, err)
			http.Error(rw, "", http.StatusInternalServerError)
		}
	}
}

func (p *Proxy) roundTripperForRestConfig(config *rest.Config) (http.RoundTripper, error) {
	// get golang tls config to the API server
	tlsConfig, err := rest.TLSConfigFor(config)
	if err != nil {
		return nil, err
	}

	// create tls transport to request
	tlsTransport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	// get kube transport config form rest client config
	restTransportConfig, err := config.TransportConfig()
	if err != nil {
		return nil, err
	}

	// wrap golang tls config with kube transport round tripper
	clientRT, err := transport.HTTPWrappersForConfig(restTransportConfig, tlsTransport)
	if err != nil {
		return nil, err
	}

	return clientRT, nil
}

// Return the proxy OIDC token authenticator
func (p *Proxy) OIDCTokenAuthenticator() authenticator.Token {
	return p.tokenAuther
}
