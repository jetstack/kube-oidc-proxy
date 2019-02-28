package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/client-go/rest"
	authtypes "k8s.io/kubernetes/pkg/apis/authentication"
)

type Proxy struct {
	reqAuther  *bearertoken.Authenticator
	restClient *rest.Config
}

func (p *Proxy) Run() error {
	logrus.Infof("waiting for oidc provider to become ready...")
	time.Sleep(10 * time.Second)
	logrus.Infof("proxy ready")

	url, err := url.Parse("https://api.jvl-cluster.develop.tarmak.org")
	if err != nil {
		logrus.Fatalf("failed to parse url: %s", err)
	}

	//transport := &http.Transport{TLSClientConfig: tlsConfig}
	proxy := httputil.NewSingleHostReverseProxy(url)
	proxy.Transport = p
	proxy.ErrorHandler = p.errorHandler

	err = http.ListenAndServeTLS(":8000", "apiserver.crt", "apiserver.key", proxy)
	if err != nil {
		log.Fatal(err)
	}

	return nil
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
		if h == authtypes.ImpersonateUserHeader ||
			h == authtypes.ImpersonateGroupHeader ||
			strings.HasPrefix(h, authtypes.ImpersonateUserExtraHeaderPrefix) {
			return nil, errImpersonateHeader
		}
	}

	name := info.User.GetName()

	// no name available to reject request
	if name == "" {
		return nil, errNoName
	}

	// set impersonation header using authenticated identity name
	r.Header.Set(authtypes.ImpersonateUserHeader, name)

	// return through normal transport layer function
	return p.restClient.Transport.RoundTrip(r)
}

func (p *Proxy) errorHandler(rw http.ResponseWriter, r *http.Request, err error) {
	switch err {

	// failed auth
	case errUnauthorized:
		logrus.Debugf("unauthenticated user request %s", r.RemoteAddr)

		rw.WriteHeader(http.StatusUnauthorized)
		if _, err := rw.Write([]byte("Unauthorized")); err != nil {
			logrus.Errorf("failed to write Unauthorized to client response: %s", err)
		}
		return

		// user request with impersonation
	case errImpersonateHeader:
		logrus.Debugf("impersonation user request %s", r.RemoteAddr)

		rw.WriteHeader(http.StatusForbidden)
		if _, err := rw.Write(
			[]byte("Impersonation requests are disabled when using kube-oidc-proxy"),
		); err != nil {
			logrus.Errorf("failed to write Unauthorized to client response: %s", err)
		}
		return

		// no name given or available in oidc response
	case errNoName:
		logrus.Debugf("no name available in oidc info %s", r.RemoteAddr)

		rw.WriteHeader(http.StatusForbidden)
		if _, err := rw.Write(
			[]byte("No name available in OIDC info response"),
		); err != nil {
			logrus.Errorf("failed to write Unauthorized to client response: %s", err)
		}

		// server or unknown error
	default:
		logrus.Errorf("unknown error (%s): %s", r.RemoteAddr, err)
		rw.WriteHeader(http.StatusBadGateway)
	}
}
