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

	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"k8s.io/klog"
	authtypes "k8s.io/kubernetes/pkg/apis/authentication"
)

var (
	errUnauthorized      = errors.New("Unauthorized")
	errImpersonateHeader = errors.New("Impersonate-User in header")
	errNoName            = errors.New("No name in OIDC info")

	// http headers are case-insensitive
	impersonateUserHeader  = strings.ToLower(authtypes.ImpersonateUserHeader)
	impersonateGroupHeader = strings.ToLower(authtypes.ImpersonateGroupHeader)
	impersonateExtraHeader = strings.ToLower(authtypes.ImpersonateUserExtraHeaderPrefix)
)

type Proxy struct {
	reqAuther         *bearertoken.Authenticator
	secureServingInfo *server.SecureServingInfo

	restConfig      *rest.Config
	clientTransport http.RoundTripper
}

func New(restConfig *rest.Config, auther *bearertoken.Authenticator,
	ssinfo *server.SecureServingInfo) *Proxy {
	return &Proxy{
		restConfig:        restConfig,
		reqAuther:         auther,
		secureServingInfo: ssinfo,
	}
}

func (p *Proxy) Run(stopCh <-chan struct{}) error {
	klog.Infof("waiting for oidc provider to become ready...")

	// get golang tls config to the API server
	tlsConfig, err := rest.TLSConfigFor(p.restConfig)
	if err != nil {
		return err
	}

	// create tls transport to request
	tlsTransport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	// get kube transport config form rest client config
	restTransportConfig, err := p.restConfig.TransportConfig()
	if err != nil {
		return err
	}

	// wrap golang tls config with kube transport round tripper
	clientRT, err := transport.HTTPWrappersForConfig(restTransportConfig, tlsTransport)
	if err != nil {
		return err
	}
	p.clientTransport = clientRT

	// get API server url
	url, err := url.Parse(p.restConfig.Host)
	if err != nil {
		return fmt.Errorf("failed to parse url: %s", err)
	}

	// set up proxy handler using proxy
	proxyHandler := httputil.NewSingleHostReverseProxy(url)
	proxyHandler.Transport = p
	proxyHandler.ErrorHandler = p.Error

	// wait for oidc auther to become ready
	time.Sleep(10 * time.Second)

	if err := p.serve(proxyHandler, stopCh); err != nil {
		return err
	}

	klog.Infof("proxy ready")

	return nil
}

func (p *Proxy) serve(proxyHandler *httputil.ReverseProxy, stopCh <-chan struct{}) error {
	// securely serve using serving config
	err := p.secureServingInfo.Serve(proxyHandler, time.Second*60, stopCh)
	if err != nil {
		return err
	}

	return nil
}

func (p *Proxy) RoundTrip(req *http.Request) (*http.Response, error) {
	// auth request and handle unauthed
	info, ok, err := p.reqAuther.AuthenticateRequest(req)
	if err != nil {
		klog.Errorf("unable to authenticate the request due to an error: %v", err)

		if !ok {
			klog.Info("not a valid OIDC token, forward to Kubernetes API as this may be a valid API token")
			return p.clientTransport.RoundTrip(req)
		}

		return nil, errUnauthorized
	}

	if !ok {
		if len(req.Header.Get("Authorization")) > 0 {
			klog.Info("unrecognised Authorization header, forward to Kubernetes API as this may still be a valid header")
			return p.clientTransport.RoundTrip(req)
		}

		klog.Info("no Authorization header in request, forward to Kubernetes API as system:anonymous")
		rt := transport.NewImpersonatingRoundTripper(transport.ImpersonationConfig{UserName: "system:anonymous"}, p.clientTransport)
		return rt.RoundTrip(req)
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

	// set impersonation header using authenticated user identity
	conf := transport.ImpersonationConfig{
		UserName: user.GetName(),
		Groups:   user.GetGroups(),
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

		// server or unknown error
	default:
		klog.Errorf("unknown error (%s): %s", r.RemoteAddr, err)
		http.Error(rw, "", http.StatusInternalServerError)
	}
}
