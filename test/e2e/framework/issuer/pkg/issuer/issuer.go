// Copyright Jetstack Ltd. See LICENSE for details.
package issuer

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	"k8s.io/klog"
)

type Issuer struct {
	issuerURL         string
	keyFile, certFile string

	sk *rsa.PrivateKey

	stopCh <-chan struct{}
}

func New(issuerURL, keyFile, certFile string, stopCh <-chan struct{}) (*Issuer, error) {
	b, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(b)
	if block == nil {
		return nil,
			fmt.Errorf("failed to parse PEM block containing the key: %q", keyFile)
	}

	sk, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return &Issuer{
		keyFile:   keyFile,
		certFile:  certFile,
		issuerURL: issuerURL,
		sk:        sk,
		stopCh:    stopCh,
	}, nil
}

func (i *Issuer) Run(bindAddress, listenPort string) (<-chan struct{}, error) {
	serveAddr := fmt.Sprintf("%s:%s", bindAddress, listenPort)

	l, err := net.Listen("tcp", serveAddr)
	if err != nil {
		return nil, err
	}

	go func() {
		<-i.stopCh
		if l != nil {
			l.Close()
		}
	}()

	compCh := make(chan struct{})
	go func() {
		defer close(compCh)

		err := http.ServeTLS(l, i, i.certFile, i.keyFile)
		if err != nil {
			klog.Errorf("stoped serving TLS (%s): %s", serveAddr, err)
		}
	}()

	klog.Infof("mock issuer listening and serving on %s", serveAddr)

	return compCh, nil
}

func (i *Issuer) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	klog.Infof("mock issuer received url %s", r.URL)

	rw.Header().Set("Content-Type", "application/json; charset=utf-8")

	switch r.URL.String() {
	case "/.well-known/openid-configuration":
		rw.WriteHeader(http.StatusOK)

		if _, err := rw.Write(i.wellKnownResponse()); err != nil {
			klog.Errorf("failed to write openid-configuration response: %s", err)
		}

	case "/certs":
		rw.WriteHeader(http.StatusOK)

		certsDiscovery := i.certsDiscovery()
		if _, err := rw.Write(certsDiscovery); err != nil {
			klog.Errorf("failed to write certificate discovery response: %s", err)
		}

	default:
		klog.Errorf("unexpected URL request: %s", r.URL)
		rw.WriteHeader(http.StatusNotFound)
		rw.Write([]byte("{}\n"))
	}
}

func (i *Issuer) wellKnownResponse() []byte {
	return []byte(fmt.Sprintf(`{
 "issuer": "%s",
 "jwks_uri": "%s/certs",
 "subject_types_supported": [
  "public"
 ],
 "id_token_signing_alg_values_supported": [
  "RS256"
 ],
 "scopes_supported": [
  "openid",
  "email"
 ],
 "token_endpoint_auth_methods_supported": [
  "client_secret_post",
  "client_secret_basic"
 ],
 "claims_supported": [
  "email",
	"e2e-username-claim",
	"e2e-groups-claim",
  "sub"
 ],
 "code_challenge_methods_supported": [
  "plain",
  "S256"
 ]
}`, i.issuerURL, i.issuerURL))
}

func (i *Issuer) certsDiscovery() []byte {
	n := base64.RawURLEncoding.EncodeToString(i.sk.N.Bytes())

	return []byte(fmt.Sprintf(`{
	  "keys": [
	    {
	      "kid": "0905d6f9cd9b0f1f852e8b207e8f673abca4bf75",
	      "e": "AQAB",
	      "kty": "RSA",
	      "alg": "RS256",
	      "n": "%s",
	      "use": "sig"
	    }
	  ]
	}`, n))
}
