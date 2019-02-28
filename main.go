package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	authv1 "k8s.io/api/authentication/v1"
	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/apiserver/plugin/pkg/authenticator/token/oidc"
)

var (
	errUnauthorized      = errors.New("Unauthorized")
	errImpersonateHeader = errors.New("Impersonate-User in header")
	errNoName            = errors.New("No name in OIDC info")
)

type Proxy struct {
	tlsTransport *http.Transport
	reqAuther    *bearertoken.Authenticator
}

func main() {
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

	config := oidc.Options{
		IssuerURL:     "https://accounts.google.com",
		ClientID:      "",
		UsernameClaim: "email",
	}

	oidcAuther, err := oidc.New(config)
	if err != nil {
		logrus.Fatal(err)
	}
	reqAuther := bearertoken.New(oidcAuther)
	logrus.Infof("waiting for oidc provider to become ready...")
	time.Sleep(10 * time.Second)
	logrus.Infof("proxy ready")

	url, err := url.Parse("https://api.jvl-cluster.develop.tarmak.org")
	if err != nil {
		logrus.Fatalf("failed to parse url: %s", err)
	}

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	p := &Proxy{transport, reqAuther}
	proxy := httputil.NewSingleHostReverseProxy(url)
	proxy.Transport = p
	proxy.ErrorHandler = p.ErrorHandler

	err = http.ListenAndServeTLS(":8000", "apiserver.crt", "apiserver.key", proxy)
	if err != nil {
		log.Fatal(err)
	}
}

func (p *Proxy) ErrorHandler(rw http.ResponseWriter, r *http.Request, err error) {
	switch err {

	case errUnauthorized:
		logrus.Debugf("unauthenticated user request %s", r.RemoteAddr)

		rw.WriteHeader(http.StatusUnauthorized)
		if _, err := rw.Write([]byte("Unauthorized")); err != nil {
			logrus.Errorf("failed to write Unauthorized to client response: %s", err)
		}
		return

	case errImpersonateHeader:
		logrus.Debugf("impersonation user request %s", r.RemoteAddr)

		rw.WriteHeader(http.StatusForbidden)
		if _, err := rw.Write(
			[]byte("Impersonation requests are disabled when using kube-oidc-proxy"),
		); err != nil {
			logrus.Errorf("failed to write Unauthorized to client response: %s", err)
		}
		return

	case errNoName:
		logrus.Debugf("no name available in oidc info %s", r.RemoteAddr)

		rw.WriteHeader(http.StatusForbidden)
		if _, err := rw.Write(
			[]byte("No name available in OIDC info response"),
		); err != nil {
			logrus.Errorf("failed to write Unauthorized to client response: %s", err)
		}

	default:
		logrus.Errorf("unknown error (%s): %s", r.RemoteAddr, err)
		rw.WriteHeader(http.StatusBadGateway)
	}
}

func (p *Proxy) RoundTrip(r *http.Request) (*http.Response, error) {
	// auth request
	info, ok, err := p.reqAuther.AuthenticateRequest(r)
	if err != nil {
		logrus.Errorf("unable to authenticate the request due to an error: %v", err)
		return nil, errUnauthorized
	}

	if !ok {
		return nil, errUnauthorized
	}

	// check for incoming impersonation headers and reject if any exists
	for h := range r.Header {
		if h == authv1.ImpersonateUserHeader ||
			h == authv1.ImpersonateGroupHeader ||
			strings.HasPrefix(h, authv1.ImpersonateUserExtraHeaderPrefix) {
			return nil, errImpersonateHeader
		}
	}

	name := info.User.GetName()

	// no name available to reject request
	if name == "" {
		return nil, errNoName
	}

	// set impersonation header using authenticated identity name
	r.Header.Set(authv1.ImpersonateUserHeader, name)

	// return through normal transport layer function
	return p.tlsTransport.RoundTrip(r)
}
