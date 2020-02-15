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

	//utilnet "k8s.io/apimachinery/pkg/util/net"
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
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/audit"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/context"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/hooks"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/tokenreview"
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
	ExternalAddress      string
}

type errorHandlerFn func(http.ResponseWriter, *http.Request, error)

type Proxy struct {
	oidcRequestAuther *bearertoken.Authenticator
	tokenAuther       authenticator.Token
	tokenReviewer     *tokenreview.TokenReview
	secureServingInfo *server.SecureServingInfo
	auditor           *audit.Audit

	restConfig            *rest.Config
	clientTransport       http.RoundTripper
	noAuthClientTransport http.RoundTripper

	config *Config

	hooks       *hooks.Hooks
	handleError errorHandlerFn
}

func New(config *Config, restConfig *rest.Config,
	oidcOptions *options.OIDCAuthenticationOptions,
	auditOptions *options.AuditOptions,
	tokenReviewer *tokenreview.TokenReview, ssinfo *server.SecureServingInfo,
) (*Proxy, error) {

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

	auditor, err := audit.New(auditOptions, config.ExternalAddress, ssinfo)
	if err != nil {
		return nil, err
	}

	return &Proxy{
		config:            config,
		hooks:             hooks.New(),
		restConfig:        restConfig,
		tokenReviewer:     tokenReviewer,
		secureServingInfo: ssinfo,
		oidcRequestAuther: bearertoken.New(tokenAuther),
		tokenAuther:       tokenAuther,
		auditor:           auditor,
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

	waitCh, err := p.serve(stopCh, proxyHandler)
	if err != nil {
		return nil, err
	}

	return waitCh, nil
}

func (p *Proxy) serve(stopCh <-chan struct{}, handler http.Handler) (<-chan struct{}, error) {
	// Set up proxy handlers
	handler = p.auditor.WithRequest(handler)
	handler = p.withImpersonateRequest(handler)
	handler = p.withAuthenticateRequest(handler)

	// Add the auditor backend as a shutdown hook
	p.hooks.AddPreShutdownHook("AuditBackend", p.auditor.Shutdown)

	// Securely serve using serving config
	waitCh, err := p.secureServingInfo.Serve(handler, time.Second*60, stopCh)
	if err != nil {
		return nil, err
	}

	return waitCh, nil
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

func (p *Proxy) withAuthenticateRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Auth request and handle unauthed
		info, ok, err := p.oidcRequestAuther.AuthenticateRequest(req)
		if err != nil {

			// Attempt to passthrough request if valid token
			if p.config.TokenReview {
				p.tokenReview(rw, req)
				return
			}

			p.handleError(rw, req, errUnauthorized)
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

func (p *Proxy) withImpersonateRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// If we have disabled impersonation we can forward the request right away
		if p.config.DisableImpersonation {
			klog.V(2).Infof("passing on request with no impersonation: %s", req.RemoteAddr)
			// Indicate we need to not use impersonation.
			req = req.WithContext(context.WithNoImpersonation(req.Context()))
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

		conf := &transport.ImpersonationConfig{
			UserName: user.GetName(),
			Groups:   groups,
			Extra:    user.GetExtra(),
		}

		// Add the impersonation configuration to the context.
		req = req.WithContext(context.WithImpersonationConfig(req.Context(), conf))
		handler.ServeHTTP(rw, req)
	})
}

func (p *Proxy) tokenReview(rw http.ResponseWriter, req *http.Request) {
	klog.V(4).Infof("attempting to validate a token in request using TokenReview endpoint(%s)",
		req.RemoteAddr)

	ok, err := p.tokenReviewer.Review(req)
	// No error so passthrough the request
	if err == nil && ok {
		klog.V(4).Infof("passing request with valid token through (%s)",
			req.RemoteAddr)

		// Set no impersonation headers and re-add removed headers.
		req = req.WithContext(context.WithNoImpersonation(req.Context()))
		return
	}

	if err != nil {
		klog.Errorf("unable to authenticate the request via TokenReview due to an error (%s): %s",
			req.RemoteAddr, err)
	}

	p.handleError(rw, req, errUnauthorized)
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

func (p *Proxy) newErrorHandler() func(rw http.ResponseWriter, r *http.Request, err error) {
	unauthedAuditor := audit.NewUnauthenticatedHandler(p.auditor)

	return func(rw http.ResponseWriter, r *http.Request, err error) {
		if err == nil {
			klog.Error("error was called with no error")
			http.Error(rw, "", http.StatusInternalServerError)
			return
		}

		switch err {

		// failed auth
		case errUnauthorized:
			klog.V(2).Infof("unauthenticated user request %s", r.RemoteAddr)

			// If Unauthorized then report to audit
			unauthedAuditor.ServeHTTP(rw, r)

			http.Error(rw, "Unauthorized", http.StatusUnauthorized)
			return

			// user request with impersonation
		case errImpersonateHeader:
			klog.V(2).Infof("impersonation user request %s", r.RemoteAddr)

			http.Error(rw, "Impersonation requests are disabled when using kube-oidc-proxy", http.StatusForbidden)
			return

			// no name given or available in oidc request
		case errNoName:
			klog.V(2).Infof("no name available in oidc info %s", r.RemoteAddr)
			http.Error(rw, "Username claim not available in OIDC Issuer response", http.StatusForbidden)
			return

			// no impersonation configuration found in context
		case errNoImpersonationConfig:
			klog.Errorf("if you are seeing this, there is likely a bug in the proxy (%s): %s", r.RemoteAddr, err)
			http.Error(rw, "", http.StatusInternalServerError)
			return

			// server or unknown error
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

func (p *Proxy) RunShutdownHooks() error {
	return p.hooks.RunPreShutdownHooks()
}
