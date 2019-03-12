// Copyright Jetstack Ltd. See LICENSE for details.
package issuer

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jetstack/kube-oidc-proxy/pkg/utils"
	"k8s.io/klog"
)

type Issuer struct {
	tlsDir            string
	listenPort        string
	certPath, keyPath string

	t *testing.T
}

func New(t *testing.T, tlsDir string) *Issuer {
	return &Issuer{
		tlsDir: tlsDir,
		t:      t,
	}
}

func (i *Issuer) Run() error {
	listenPort, err := utils.FreePort()
	if err != nil {
		return err
	}
	i.listenPort = listenPort

	certPath, keyPath, err := utils.NewTLSSelfSignedCertKey(i.tlsDir, "oidc-issuer")
	if err != nil {
		return err
	}
	i.certPath = certPath
	i.keyPath = keyPath

	serveAddr := fmt.Sprintf("127.0.0.1:%s", i.listenPort)

	go func() {
		err = http.ListenAndServeTLS(serveAddr, i.certPath, i.keyPath, i)
		if err != nil {
			i.t.Errorf("failed to server secure tls: %s", err)
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
		rw.Write(i.wellKnownResponse())

	default:
		i.t.Errorf("unexpected URL request: %s", r.URL)
	}
}

func (i *Issuer) CertPath() string {
	return i.certPath
}

func (i *Issuer) KeyPath() string {
	return i.keyPath
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
  "sub"
 ],
 "code_challenge_methods_supported": [
  "plain",
  "S256"
 ]
}`, i.listenPort, i.listenPort))
}
