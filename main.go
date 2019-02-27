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
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/apiserver/plugin/pkg/authenticator/token/oidc"
)

var (
	errUnauthorized = errors.New("Unauthorized")
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
		UsernameClaim: "sub",
	}

	iodcAuther, err := oidc.New(config)
	if err != nil {
		logrus.Fatal(err)
	}
	reqAuther := bearertoken.New(iodcAuther)
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
	//proxy.Transport = transport
	proxy.Transport = p
	proxy.ErrorHandler = p.ErrorHandler

	err = http.ListenAndServeTLS(":8000", "apiserver.crt", "apiserver.key", proxy)
	if err != nil {
		log.Fatal(err)
	}
}

func setHeader(w http.ResponseWriter, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
}

func duplicateHeader(o http.Header) http.Header {
	n := http.Header{}
	for k, v := range o {
		n[k] = v
	}

	return n
}

func (p *Proxy) ErrorHandler(rw http.ResponseWriter, r *http.Request, err error) {
	if err == errUnauthorized {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	rw.WriteHeader(http.StatusBadGateway)
}

func (p *Proxy) RoundTrip(r *http.Request) (*http.Response, error) {
	_, ok, err := p.reqAuther.AuthenticateRequest(r)
	if err != nil {
		logrus.Errorf("Unable to authenticate the request due to an error: %v", err)
		return nil, errUnauthorized
	}
	if !ok {
		return nil, errUnauthorized
	}

	return p.tlsTransport.RoundTrip(r)
}
