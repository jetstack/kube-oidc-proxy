package main

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
	"k8s.io/klog"
	authtypes "k8s.io/kubernetes/pkg/apis/authentication"
)

const (
	errUnauthorized      = "Unauthorized"
	errImpersonateHeader = "Impersonate-User in header"
	errNoName            = "No name in OIDC info"
)

type Proxy struct {
	reqAuther         *bearertoken.Authenticator
	secureServingInfo *server.SecureServingInfo

	restConfig      *rest.Config
	clientTransport http.RoundTripper
}

func (p *Proxy) Run(stopCh <-chan struct{}) error {
	klog.Infof("waiting for oidc provider to become ready...")

	// get client transport to the API server
	tlsConfig, err := rest.TLSConfigFor(p.restConfig)
	if err != nil {
		return err
	}
	tlsConfig.BuildNameToCertificate()

	p.clientTransport = &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	// get API server url
	url, err := url.Parse(p.restConfig.Host)
	if err != nil {
		return fmt.Errorf("failed to parse url: %s", err)
	}

	// set up proxy handler using client config
	proxyHandler := httputil.NewSingleHostReverseProxy(url)
	proxyHandler.Transport = p
	proxyHandler.ErrorHandler = p.Error

	// wait for oidc auther to become ready
	time.Sleep(10 * time.Second)

	// securely serve using serving config
	err = p.secureServingInfo.Serve(proxyHandler, time.Second*120, stopCh)
	if err != nil {
		return err
	}

	klog.Infof("proxy ready")

	return nil
}

func (p *Proxy) RoundTrip(req *http.Request) (*http.Response, error) {
	// auth request and handle unauthed
	info, ok, err := p.reqAuther.AuthenticateRequest(req)
	if err != nil {
		klog.Errorf("unable to authenticate the request due to an error: %v", err)
		return nil, errors.New(errUnauthorized)
	}

	if !ok {
		return nil, errors.New(errUnauthorized)
	}

	// check for incoming impersonation headers and reject if any exists
	for h := range req.Header {
		if h == authtypes.ImpersonateUserHeader ||
			h == authtypes.ImpersonateGroupHeader ||
			strings.HasPrefix(h, authtypes.ImpersonateUserExtraHeaderPrefix) {
			return nil, errors.New(errImpersonateHeader)
		}
	}

	name := info.User.GetName()

	// no name available so reject request
	if name == "" {
		return nil, errors.New(errNoName)
	}

	// set impersonation header using authenticated identity name
	req.Header.Set(authtypes.ImpersonateUserHeader, name)

	// push request through our TLS client to the API server
	return p.clientTransport.RoundTrip(req)
}

func (p *Proxy) Error(rw http.ResponseWriter, r *http.Request, err error) {
	switch err.Error() {

	// failed auth
	case errUnauthorized:
		klog.V(2).Infof("unauthenticated user request %s", r.RemoteAddr)

		rw.WriteHeader(http.StatusUnauthorized)
		if _, err := rw.Write([]byte("Unauthorized")); err != nil {
			klog.Errorf(
				"failed to write Unauthorized to client response (%s): %s", r.RemoteAddr, err)
		}
		return

		// user request with impersonation
	case errImpersonateHeader:
		klog.V(2).Infof("impersonation user request %s", r.RemoteAddr)

		rw.WriteHeader(http.StatusForbidden)
		if _, err := rw.Write(
			[]byte("Impersonation requests are disabled when using kube-oidc-proxy"),
		); err != nil {
			klog.Errorf("failed to write Unauthorized to client response: %s", err)
		}
		return

		// no name given or available in oidc response
	case errNoName:
		klog.V(2).Infof("no name available in oidc info %s", r.RemoteAddr)

		rw.WriteHeader(http.StatusForbidden)
		if _, err := rw.Write(
			[]byte("No name available in OIDC info response"),
		); err != nil {
			klog.Errorf("failed to write Unauthorized to client response: %s", err)
		}

		// server or unknown error
	default:
		klog.Errorf("unknown error (%s): %s", r.RemoteAddr, err)
		rw.WriteHeader(http.StatusBadGateway)
	}
}
