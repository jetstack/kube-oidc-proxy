// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"testing"
	"time"

	jose "gopkg.in/square/go-jose.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog"

	// required to register oidc auth plugin for rest client
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"github.com/jetstack/kube-oidc-proxy/pkg/e2e/issuer"
	"github.com/jetstack/kube-oidc-proxy/pkg/util"
)

const (
	readinessProbeURL = "http://127.0.0.1:8080/ready"
)

type E2E struct {
	kubeclient     *kubernetes.Clientset
	kubeKubeconfig string

	signer    jose.Signer
	wrappedRT *wraperRT
	issuer    *issuer.Issuer

	proxyClient    *http.Client
	proxyCmd       *exec.Cmd
	proxyPort      string
	proxyTransport *http.Transport

	proxyKeyCertPair *util.KeyCertPair

	tmpDir string
}

type wraperRT struct {
	transport http.RoundTripper
	token     string
}

func (w *wraperRT) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Add("Authorization", fmt.Sprintf("bearer %s", w.token))
	return w.transport.RoundTrip(r)
}

func New(kubeconfig, tmpDir string,
	kubeclient *kubernetes.Clientset) *E2E {
	return &E2E{
		kubeclient:     kubeclient,
		tmpDir:         tmpDir,
		kubeKubeconfig: kubeconfig,
	}
}

func (e *E2E) Run() error {
	proxyTransport, err := e.newIssuerProxyPair()
	if err != nil {
		return err
	}
	e.proxyTransport = proxyTransport

	e.proxyClient = http.DefaultClient
	e.wrappedRT = &wraperRT{
		transport: proxyTransport,
	}
	e.proxyClient.Transport = e.wrappedRT

	return nil
}

func (e *E2E) testToken(t *testing.T, payload []byte, target string, expCode int, expBody string) {
	signedToken, err := e.signToken(payload)
	if err != nil {
		t.Error(err)
		return
	}

	e.wrappedRT.token = signedToken

	resp, err := e.proxyClient.Get(target)
	if err != nil {
		t.Error(err)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
		return
	}

	body = bytes.TrimSpace(body)

	if resp.StatusCode != expCode {
		t.Errorf("got unexpected status code (%s), exp=%d got=%d",
			target, expCode, resp.StatusCode)

		if len(expBody) > 0 {
			t.Errorf("got body='%s'", body)
		}
	}

	if len(expBody) == 0 {
		return
	}

	r, err := regexp.Compile(expBody)
	if err != nil {
		t.Error(err)
		return
	}

	if !r.Match(body) {
		t.Errorf("got unexpected response body (%s)\nexp='%s'\ngot='%s'",
			target, expBody, body)
	}
}

func (e *E2E) newIssuerProxyPair() (*http.Transport, error) {
	pairTmpDir, err := ioutil.TempDir(e.tmpDir, "")
	if err != nil {
		return nil, err
	}

	issuer := issuer.New(pairTmpDir)
	if err := issuer.Run(); err != nil {
		return nil, err
	}
	e.issuer = issuer

	pkcp, err := util.NewTLSSelfSignedCertKey(pairTmpDir, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create key pair: %s", err)
	}
	e.proxyKeyCertPair = pkcp

	signer, err := jose.NewSigner(jose.SigningKey{
		Algorithm: jose.SignatureAlgorithm("RS256"),
		Key:       issuer.KeyCertPair().Key,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise new jwt signer: %s", err)
	}
	e.signer = signer

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(pkcp.Cert); !ok {
		return nil, fmt.Errorf("failed to append proxy cert data to cert pool %s", pkcp.CertPath)
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: certPool,
		},
	}

	proxyPort, err := util.FreePort()
	if err != nil {
		return nil, err
	}
	e.proxyPort = proxyPort

	cmd := exec.Command("../../kube-oidc-proxy",
		fmt.Sprintf("--oidc-issuer-url=https://127.0.0.1:%s", issuer.Port()),
		fmt.Sprintf("--oidc-ca-file=%s", e.issuer.KeyCertPair().CertPath),
		"--oidc-client-id=kube-oidc-proxy_e2e_client-id",
		"--oidc-username-claim=e2e-username-claim",
		"--oidc-groups-claim=e2e-groups-claim",
		"--oidc-signing-algs=RS256",

		"--bind-address=127.0.0.1",
		fmt.Sprintf("--secure-port=%s", e.proxyPort),
		fmt.Sprintf("--tls-cert-file=%s", e.proxyKeyCertPair.CertPath),
		fmt.Sprintf("--tls-private-key-file=%s", e.proxyKeyCertPair.KeyPath),

		fmt.Sprintf("--kubeconfig=%s", e.kubeKubeconfig),
	)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	e.proxyCmd = cmd

	_, err = verifyProxyReadinessPoll(readinessProbeURL, 2*time.Second, 30*time.Second)
	if err != nil {
		return nil, err
	}

	return transport, nil
}

func (e *E2E) validToken() []byte {
	// valid for 10 mins
	return []byte(fmt.Sprintf(`{
	"iss":"https://127.0.0.1:%s",
	"aud":["kube-oidc-proxy_e2e_client-id","aud-2"],
	"e2e-username-claim":"test-username",
	"e2e-groups-claim":["group-1","group-2"],
	"exp":%d
	}`, e.issuer.Port(), time.Now().Add(time.Minute*10).Unix()))
}

func (e *E2E) signToken(token []byte) (string, error) {
	jwt, err := e.signer.Sign(token)
	if err != nil {
		return "", fmt.Errorf("failed to sign jwt: %s", err)
	}

	signedToken, err := jwt.CompactSerialize()
	if err != nil {
		return "", err
	}

	return signedToken, nil
}

func (e *E2E) proxyRestClient() (*rest.Config, error) {
	// valid signed token for auth to proxy
	signedToken, err := e.signToken(e.validToken())
	if err != nil {
		return nil, err
	}

	// rest config pointed to proxy
	return &rest.Config{
		Host: fmt.Sprintf("https://127.0.0.1:%s", e.proxyPort),
		AuthProvider: &clientcmdapi.AuthProviderConfig{
			Name: "oidc",
			Config: map[string]string{
				"client-id":      "kube-oidc-proxy_e2e_client-id",
				"id-token":       signedToken,
				"idp-issuer-url": "https://127.0.0.1:" + e.proxyPort,
			},
		},
		TLSClientConfig: rest.TLSClientConfig{
			CAData: e.proxyKeyCertPair.Cert,
		},

		APIPath: "/api",
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &corev1.SchemeGroupVersion,
			NegotiatedSerializer: scheme.Codecs,
		},
	}, nil
}

func (e *E2E) cleanup() {
	if e.proxyCmd != nil &&
		e.proxyCmd.Process != nil {

		if err := e.proxyCmd.Process.Kill(); err != nil {
			klog.Errorf("failed to kill kube-oidc-proxy process: %s",
				err)
		}

		e.proxyCmd = nil
	}
}

func (e *E2E) runProxy(extraArgs ...string) error {
	if e.issuer == nil {
		return errors.New("failed to run proxy: issuer not ready")
	}

	args := append(
		[]string{
			fmt.Sprintf("--oidc-issuer-url=https://127.0.0.1:%s", e.issuer.Port()),
			fmt.Sprintf("--oidc-ca-file=%s", e.issuer.KeyCertPair().CertPath),
			"--oidc-client-id=kube-oidc-proxy_e2e_client-id",
			"--oidc-username-claim=e2e-username-claim",
			"--oidc-groups-claim=e2e-groups-claim",
			"--oidc-signing-algs=RS256",
			"--v=5",

			"--bind-address=127.0.0.1",
			fmt.Sprintf("--secure-port=%s", e.proxyPort),
			fmt.Sprintf("--tls-cert-file=%s", e.proxyKeyCertPair.CertPath),
			fmt.Sprintf("--tls-private-key-file=%s", e.proxyKeyCertPair.KeyPath),

			fmt.Sprintf("--kubeconfig=%s", e.kubeKubeconfig),
		},
		extraArgs...,
	)

	cmd := exec.Command("../../kube-oidc-proxy", args...)

	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		return err
	}

	e.proxyCmd = cmd

	_, err := verifyProxyReadinessPoll(readinessProbeURL, 2*time.Second, 30*time.Second)
	if err != nil {
		return err
	}

	return nil
}
