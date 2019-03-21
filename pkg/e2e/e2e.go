// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"k8s.io/client-go/rest"

	// required to register oidc auth plugin for rest client
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"github.com/jetstack/kube-oidc-proxy/pkg/e2e/issuer"
	"github.com/jetstack/kube-oidc-proxy/pkg/utils"
)

type E2E struct {
	apiserverCnf *rest.Config
	t            *testing.T

	proxyKubeconfig string
	tmpDir          string
}

type token struct {
	header, payload, sig string
}

type wraperRT struct {
	transport http.RoundTripper
	token     *token
}

func (w *wraperRT) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("Authorization", fmt.Sprintf("bearer %s", w.token))
	return w.transport.RoundTrip(r)
}

func New(t *testing.T, kubeconfig, tmpDir string,
	apiserverCnf *rest.Config) *E2E {
	return &E2E{
		apiserverCnf:    apiserverCnf,
		t:               t,
		tmpDir:          tmpDir,
		proxyKubeconfig: kubeconfig,
	}
}

func (e *E2E) Run() {
	//proxy, issuer, clientKubeconfig, err := e.newIssuerProxyPair()
	proxyCmd, issuer, proxyTransport, proxyPort, err := e.newIssuerProxyPair()
	if err != nil {
		e.t.Error(err)
		return
	}

	client := http.DefaultClient
	wrappedRT := &wraperRT{
		transport: proxyTransport,
	}
	client.Transport = wrappedRT

	validToken := &token{
		header: "some header",
		payload: fmt.Sprintf(`{
"iss":"https://127.0.0.1:%s",
"aud":["test-client-1","kube-oidc-proy_e2e_client-id"],
"exp":%d,
"alg":"RS256"
}`, issuer.Port(), time.Now().Add(time.Minute).Unix()),
	}

	b, err := ioutil.ReadFile(issuer.KeyPath())
	if err != nil {
		e.t.Error(err)
		return
	}

	b = bytes.TrimSpace(b)
	p, rest := pem.Decode(b)
	if len(rest) != 0 {
		e.t.Errorf("got rest decoding pem file %s: %s",
			issuer.KeyPath(), rest)
		return
	}

	sk, err := x509.ParsePKCS1PrivateKey(p.Bytes)
	if err != nil {
		e.t.Error(err)
		return
	}

	hashed := sha256.Sum256([]byte(validToken.payload))
	signature, err := rsa.SignPKCS1v15(rand.Reader, sk, crypto.SHA256, hashed[:])
	if err != nil {
		e.t.Error(err)
		return
	}

	validToken.sig = fmt.Sprintf(`{
"signature":"%s"
}`, validToken.encode(string(signature)))

	wrappedRT.token = validToken

	resp, err := http.Get(fmt.Sprintf("https://127.0.0.1:%s/api/v1/nodes", proxyPort))
	if err != nil {
		e.t.Error(err)
		return
	}

	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		e.t.Error(err)
		return
	}

	fmt.Printf(">>%v\n", b)
	fmt.Printf(">>%v\n", resp.StatusCode)

	tests := []struct {
		header, payload, sig string
		expCode              int
		expBody              string
	}{

		// no bearer token
		{
			expCode: 401,
			expBody: "Unauthorized\n",
		},

		// invalid bearer token
		{
			header:  "bad-header",
			payload: "bad-payload",
			sig:     "bad-sig",
			expCode: 401,
			expBody: "Unauthorized\n",
		},

		// wrong issuer
		{
			header:  "{}",
			payload: `{"iss":"incorrect issuer"}`,
			sig:     "bad-sig",
			expCode: 401,
			expBody: "Unauthorized\n",
		},

		// correct issuer, no audience
		{
			header:  "{}",
			payload: fmt.Sprintf(`{"iss":"https://127.0.0.1:%s"}`, issuer.Port()),
			sig:     "bad-sig",
			expCode: 401,
			expBody: "Unauthorized\n",
		},

		// wrong audience
		{
			header: "{}",
			payload: fmt.Sprintf(`{
"iss":"https://127.0.0.1:%s",
"aud":["test-client-1","test-client-2"]
}`, issuer.Port()),
			sig:     "bad-sig",
			expCode: 401,
			expBody: "Unauthorized\n",
		},

		// correct audience
		{
			header: "{}",
			payload: fmt.Sprintf(`{
"iss":"https://127.0.0.1:%s",
"aud":["test-client-1","kube-oidc-proy_e2e_client-id"]
}`, issuer.Port()),
			sig:     "bad-sig",
			expCode: 401,
			expBody: "Unauthorized\n",
		},

		// token expires now
		{
			header: "{}",
			payload: fmt.Sprintf(`{
"iss":"https://127.0.0.1:%s",
"aud":["test-client-1","kube-oidc-proy_e2e_client-id"],
"exp":%d
}`, issuer.Port(), time.Now().Unix()),
			sig:     "bad-sig",
			expCode: 401,
			expBody: "Unauthorized\n",
		},

		// token in date
		{
			header: "{}",
			payload: fmt.Sprintf(`{
"iss":"https://127.0.0.1:%s",
"aud":["test-client-1","kube-oidc-proy_e2e_client-id"],
"exp":%d
}`, issuer.Port(), time.Now().Add(time.Minute).Unix()),
			sig:     "bad-sig",
			expCode: 401,
			expBody: "Unauthorized\n",
		},

		// wrong signature algorithm
		{
			header: "{}",
			payload: fmt.Sprintf(`{
"iss":"https://127.0.0.1:%s",
"aud":["test-client-1","kube-oidc-proy_e2e_client-id"],
"exp":%d,
"alg":"foo"
}`, issuer.Port(), time.Now().Add(time.Minute).Unix()),
			sig:     "bad-sig",
			expCode: 401,
			expBody: "Unauthorized\n",
		},
	}

	for _, test := range tests {
		wrappedRT.token = &token{
			header:  test.header,
			payload: test.payload,
			sig:     test.sig,
		}

		resp, err := http.Get(fmt.Sprintf("https://127.0.0.1:%s/api/v1/nodes", proxyPort))
		if err != nil {
			e.t.Error(err)
			return
		}

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			e.t.Error(err)
			return
		}

		if resp.StatusCode != test.expCode {
			e.t.Errorf("unexpected status code from request (%s.%s.%s), exp=%d got=%d",
				test.header, test.payload, test.sig,
				test.expCode, resp.StatusCode)
		}

		if string(b) != test.expBody {
			e.t.Errorf("unexpected response body from request (%s.%s.%s), exp=%s got=%s",
				test.header, test.payload, test.sig,
				test.expBody, string(b))
		}
	}

	proxyCmd.Process.Kill()
}

func (e *E2E) newIssuerProxyPair() (*exec.Cmd, *issuer.Issuer, *http.Transport, string, error) {
	pairTmpDir, err := ioutil.TempDir(e.tmpDir, "")
	if err != nil {
		return nil, nil, nil, "", err
	}

	issuer := issuer.New(e.t, pairTmpDir)
	if err := issuer.Run(); err != nil {
		return nil, nil, nil, "", err
	}

	proxyCertPath, proxyKeyPath, err := utils.NewTLSSelfSignedCertKey(pairTmpDir, "")
	if err != nil {
		return nil, nil, nil, "", err
	}

	certPool := x509.NewCertPool()
	proxyCertData, err := ioutil.ReadFile(proxyCertPath)
	if err != nil {
		return nil, nil, nil, "", err
	}

	if ok := certPool.AppendCertsFromPEM(proxyCertData); !ok {
		return nil, nil, nil, "", fmt.Errorf("failed to append proxy cert data to cert pool %s", proxyCertPath)
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: certPool,
		},
	}

	proxyPort, err := utils.FreePort()
	if err != nil {
		return nil, nil, nil, "", err
	}

	cmd := exec.Command("../../kube-oidc-proxy",
		fmt.Sprintf("--oidc-issuer-url=https://127.0.0.1:%s", issuer.Port()),
		fmt.Sprintf("--oidc-ca-file=%s", issuer.CertPath()),
		"--oidc-client-id=kube-oidc-proy_e2e_client-id",
		"--oidc-username-claim=e2e-username-claim",

		"--bind-address=127.0.0.1",
		fmt.Sprintf("--secure-port=%s", proxyPort),
		fmt.Sprintf("--tls-cert-file=%s", proxyCertPath),
		fmt.Sprintf("--tls-private-key-file=%s", proxyKeyPath),

		fmt.Sprintf("--kubeconfig=%s", e.proxyKubeconfig),

		"-v=10",
	)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, "", err
	}

	time.Sleep(time.Second * 13)

	return cmd, issuer, transport, proxyPort, nil
}

func (e *E2E) clientKubeconfig(caPath, port string) string {
	return fmt.Sprintf(`apiVersion: v1
clusters:
- cluster:
    certificate-authority: %s
    server: https://127.0.0.1:%s
  name: kube-oidc-proxy
contexts:
- context:
    cluster: kube-oidc-proxy
    user: test-user
  name: test
kind: Config
preferences: {}
current-context: test
users:
- name: test-user
  user:`, caPath, port)
}

func (t *token) encode(part string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(part))
}

func (t *token) String() string {
	return t.encode(t.header) + "." + t.encode(t.payload) + "." + t.encode(t.sig)
}
