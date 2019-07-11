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
	"k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"k8s.io/klog"
)

var (
	errUnauthorized      = errors.New("Unauthorized")
	errImpersonateHeader = errors.New("Impersonate-User in header")
	errNoName            = errors.New("No name in OIDC info")
	errTokenParse        = errors.New("no bearer token found in request header")

	// http headers are case-insensitive
	impersonateUserHeader  = strings.ToLower(transport.ImpersonateUserHeader)
	impersonateGroupHeader = strings.ToLower(transport.ImpersonateGroupHeader)
	impersonateExtraHeader = strings.ToLower(transport.ImpersonateUserExtraHeaderPrefix)
)

type Proxy struct {
	oidcAuther *bearertoken.Authenticator
	saAuther   authenticator.Token

	secureServingInfo *server.SecureServingInfo

	restConfig      *rest.Config
	clientTransport http.RoundTripper
}

func New(restConfig *rest.Config, ssinfo *server.SecureServingInfo,
	oidcAuther *bearertoken.Authenticator, saAuther authenticator.Token) *Proxy {
	return &Proxy{
		restConfig:        restConfig,
		oidcAuther:        oidcAuther,
		secureServingInfo: ssinfo,
		saAuther:          saAuther,
	}
}

func (p *Proxy) Run(stopCh <-chan struct{}) (<-chan struct{}, error) {
	klog.Infof("waiting for oidc provider to become ready...")

	proxyHandler, err := p.initServing()
	if err != nil {
		return nil, err
	}

	waitCh, err := p.serve(proxyHandler, stopCh)
	if err != nil {
		return nil, err
	}

	// wait for oidc auther to become ready
	time.Sleep(10 * time.Second)
	klog.Infof("proxy ready")

	return waitCh, nil
}

func (p *Proxy) serve(proxyHandler *httputil.ReverseProxy, stopCh <-chan struct{}) (<-chan struct{}, error) {
	// securely serve using serving config
	waitCh, err := p.secureServingInfo.Serve(proxyHandler, time.Second*60, stopCh)
	if err != nil {
		return nil, err
	}

	return waitCh, nil
}

func (p *Proxy) RoundTrip(req *http.Request) (*http.Response, error) {
	// auth request and handle unauthed
	info, ok, err := p.oidcAuther.AuthenticateRequest(req)
	if err != nil {

		// attempt to pass through request if valid service account
		if p.saAuther != nil {
			klog.V(5).Infof("attempting to validate a service account token in request (%s)",
				req.RemoteAddr)

			saErr := p.validateServiceAccountToken(req)
			// no error so passthrough the request
			if saErr == nil {
				klog.V(5).Infof("passing request with valid service account token through (%s)",
					req.RemoteAddr)
				return p.clientTransport.RoundTrip(req)
			}

			err = fmt.Errorf("%s, %s", err, saErr)
		}

		klog.Errorf("unable to authenticate the request due to an error (%s): %s",
			req.RemoteAddr, err)
		return nil, errUnauthorized
	}

	if !ok {
		return nil, errUnauthorized
	}

	// check for incoming impersonation headers and reject if any exists
	if p.hasImpersonation(req.Header) {
		return nil, errImpersonateHeader
	}

	user := info.User

	// no name available so reject request
	if user.GetName() == "" {
		return nil, errNoName
	}

	// ensure group contains allauthenticated builtin
	found := false
	groups := user.GetGroups()
	for _, elem := range groups {
		if elem == authuser.AllAuthenticated {
			found = true
			break
		}
	}
	if !found {
		groups = append(groups, authuser.AllAuthenticated)
	}

	// set impersonation header using authenticated user identity
	conf := transport.ImpersonationConfig{
		UserName: user.GetName(),
		Groups:   groups,
		Extra:    user.GetExtra(),
	}

	rt := transport.NewImpersonatingRoundTripper(conf, p.clientTransport)

	// push request through round trippers to the API server
	return rt.RoundTrip(req)
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

func (p *Proxy) Error(rw http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		klog.Error("error was called with no error")
		http.Error(rw, "", http.StatusInternalServerError)
		return
	}

	switch err {

	// failed auth
	case errUnauthorized:
		klog.V(2).Infof("unauthenticated user request %s", r.RemoteAddr)
		http.Error(rw, "Unauthorized", http.StatusUnauthorized)
		return

		// user request with impersonation
	case errImpersonateHeader:
		klog.V(2).Infof("impersonation user request %s", r.RemoteAddr)

		http.Error(rw, "Impersonation requests are disabled when using kube-oidc-proxy", http.StatusForbidden)
		return

		// no name given or available in oidc response
	case errNoName:
		klog.V(2).Infof("no name available in oidc info %s", r.RemoteAddr)
		http.Error(rw, "Username claim not available in OIDC Issuer response", http.StatusForbidden)
		return

		// server or unknown error
	default:
		klog.Errorf("unknown error (%s): %s", r.RemoteAddr, err)
		http.Error(rw, "", http.StatusInternalServerError)
	}
}

func (p *Proxy) initServing() (*httputil.ReverseProxy, error) {
	// get golang tls config to the API server
	tlsConfig, err := rest.TLSConfigFor(p.restConfig)
	if err != nil {
		return nil, err
	}

	// create tls transport to request
	tlsTransport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	// get kube transport config form rest client config
	restTransportConfig, err := p.restConfig.TransportConfig()
	if err != nil {
		return nil, err
	}

	// wrap golang tls config with kube transport round tripper
	clientRT, err := transport.HTTPWrappersForConfig(restTransportConfig, tlsTransport)
	if err != nil {
		return nil, err
	}
	p.clientTransport = clientRT

	// get API server url
	url, err := url.Parse(p.restConfig.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %s", err)
	}

	// set up proxy handler using proxy
	proxyHandler := httputil.NewSingleHostReverseProxy(url)
	proxyHandler.Transport = p
	proxyHandler.ErrorHandler = p.Error

	return proxyHandler, nil
}
