package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	authtypes "k8s.io/kubernetes/pkg/apis/authentication"
)

var (
	errUnauthorized      = errors.New("Unauthorized")
	errImpersonateHeader = errors.New("Impersonate-User in header")
	errNoName            = errors.New("No name in OIDC info")
)

type Proxy struct {
	reqAuther         *bearertoken.Authenticator
	secureServingInfo *server.SecureServingInfo

	restClient      *rest.Config
	clientTransport http.RoundTripper
}

func (p *Proxy) Run(stopCh <-chan struct{}) error {
	klog.Infof("waiting for oidc provider to become ready...")

	// get client transport to the API server
	//transport, err := rest.TransportFor(p.restClient)
	//if err != nil {
	//	return err
	//}
	//p.clientTransport = transport

	cert, err := tls.LoadX509KeyPair("client.crt", "client.key")
	if err != nil {
		logrus.Fatal(err)
	}

	// Load CA cert
	caCert, err := ioutil.ReadFile("client.ca")
	if err != nil {
		logrus.Fatal(err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// Setup HTTPS client
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}
	tlsConfig.BuildNameToCertificate()

	p.clientTransport = &http.Transport{TLSClientConfig: tlsConfig}

	// get API server url
	url, err := url.Parse(p.restClient.Host)
	if err != nil {
		return fmt.Errorf("failed to parse url: %s", err)
	}

	// set up proxy handler using client config
	proxyHandler := httputil.NewSingleHostReverseProxy(url)
	proxyHandler.Transport = p
	proxyHandler.ErrorHandler = p.Error

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
		return nil, errUnauthorized
	}

	if !ok {
		return nil, errUnauthorized
	}

	// check for incoming impersonation headers and reject if any exists
	for h := range req.Header {
		if h == authtypes.ImpersonateUserHeader ||
			h == authtypes.ImpersonateGroupHeader ||
			strings.HasPrefix(h, authtypes.ImpersonateUserExtraHeaderPrefix) {
			return nil, errImpersonateHeader
		}
	}

	name := info.User.GetName()

	// no name available so reject request
	if name == "" {
		return nil, errNoName
	}

	// set impersonation header using authenticated identity name
	req.Header.Set(authtypes.ImpersonateUserHeader, name)

	return p.clientTransport.RoundTrip(req)
}

func (p *Proxy) Error(rw http.ResponseWriter, r *http.Request, err error) {
	switch err {

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
