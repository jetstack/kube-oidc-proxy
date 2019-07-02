// Copyright Jetstack Ltd. See LICENSE for details.
package issuer

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"k8s.io/klog"

	"github.com/jetstack/kube-oidc-proxy/pkg/utils"
)

type Issuer struct {
	tlsDir     string
	listenPort string

	keyCertPair *utils.KeyCertPair
}

func New(tlsDir string) *Issuer {
	return &Issuer{
		tlsDir: tlsDir,
	}
}

func (i *Issuer) Run() error {
	listenPort, err := utils.FreePort()
	if err != nil {
		return err
	}
	i.listenPort = listenPort

	kcp, err := utils.NewTLSSelfSignedCertKey(i.tlsDir, "oidc-issuer")
	if err != nil {
		return fmt.Errorf("failed to create issuer key pair: %s", err)
	}
	i.keyCertPair = kcp

	serveAddr := fmt.Sprintf("127.0.0.1:%s", i.listenPort)

	go func() {
		err = http.ListenAndServeTLS(serveAddr, i.keyCertPair.CertPath, i.keyCertPair.KeyPath, i)
		if err != nil {
			klog.Errorf("failed to server secure tls: %s", err)
		}
	}()

	time.Sleep(time.Second * 2)

	klog.Infof("mock issuer listening and serving on %s", serveAddr)

	return nil
}

func (i *Issuer) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	klog.Infof("mock issuer received url %s", r.URL)

	switch r.URL.String() {
	case "/.well-known/openid-configuration":
		rw.Header().Set("Content-Type", "application/json")
		if _, err := rw.Write(i.wellKnownResponse()); err != nil {
			klog.Errorf("failed to write openid-configuration response: %s", err)
		}

	case "/certs":
		rw.Header().Set("Content-Type", "application/json")

		discCerts := i.CertsDisc()
		if _, err := rw.Write(discCerts); err != nil {
			klog.Errorf("failed to write certificate discovery response: %s", err)
		}

	default:
		klog.Errorf("unexpected URL request: %s", r.URL)
	}
}

func (i *Issuer) KeyCertPair() *utils.KeyCertPair {
	return i.keyCertPair
}

func (i *Issuer) Port() string {
	return i.listenPort
}

func (i *Issuer) wellKnownResponse() []byte {
	return []byte(fmt.Sprintf(`{
 "issuer": "https://127.0.0.1:%s",
 "jwks_uri": "https://127.0.0.1:%s/certs",
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
}`, i.listenPort, i.listenPort))
}

func (i *Issuer) CertsDisc() []byte {
	n := base64.RawURLEncoding.EncodeToString(i.keyCertPair.Key.N.Bytes())

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
