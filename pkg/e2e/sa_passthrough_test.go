// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/square/go-jose.v2/jwt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiserverserviceaccount "k8s.io/apiserver/pkg/authentication/serviceaccount"

	serviceaccount "github.com/jetstack/kube-oidc-proxy/pkg/proxy/serviceaccount/authenticator"
)

const (
	namespaceSAPassthroughTest = "kube-oidc-proxy-e2e-sa-passthrough"
)

type passthroughTest struct {
	body []byte
	code int

	tokenGen func() (string, error)
}

func Test_SAPassthrough(t *testing.T) {
	e2eSuite.skipNotReady(t)
	e2eSuite.cleanup()

	err := e2eSuite.runProxy(
		"--service-account-token-passthrough",
		"--service-account-key-file="+filepath.Join(e2eSuite.tmpDir, "sa.pub"),
		"--service-account-lookup=false",
		"--service-account-issuer=kubernetes/serviceaccount",
		"--service-account-max-token-expiration=20m",
	)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	coreclient := e2eSuite.kubeclient.CoreV1()
	_, err = coreclient.Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceSAPassthroughTest,
		},
	})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	skBytes, err := ioutil.ReadFile(
		filepath.Join(e2eSuite.tmpDir, "sa.key"))
	if err != nil {
		t.Errorf("failed to read service account private key file: %s", err)
		t.FailNow()
	}

	block, _ := pem.Decode(skBytes)
	if block == nil {
		t.Errorf("failed to decode private key pem block: %s", string(skBytes))
		t.FailNow()
	}

	sk, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Errorf("failed to parse private key: %s", err)
		t.FailNow()
	}

	gener, err := serviceaccount.JWTTokenGenerator("kubernetes/serviceaccount", sk)
	if err != nil {
		t.Errorf("failed to create SA token generator: %s", err)
	}

	now := time.Now()
	sc := &jwt.Claims{
		Subject:   apiserverserviceaccount.MakeUsername(namespaceSAPassthroughTest, "test-serviceaccount"),
		Audience:  jwt.Audience([]string{"api"}),
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		Expiry:    jwt.NewNumericDate(now.Add(time.Minute)),
	}
	sav1 := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-serviceaccount",
			Namespace: namespaceSAPassthroughTest,
			UID:       "test-serviceaccount-uid",
		},
	}
	secv1 := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mysec",
			Namespace: namespaceSAPassthroughTest,
		},
	}

	sa := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-serviceaccount",
			Namespace: namespaceSAPassthroughTest,
			UID:       "test-serviceaccount-uid",
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceSAPassthroughTest,
			Name:      "mypod",
			UID:       "mypod-uid",
		},
	}
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mysec",
			Namespace: namespaceSAPassthroughTest,
		},
	}

	unauthorizedBody := []byte("Unauthorized\n")
	forbiddenBody := []byte(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Failure","message":"namespaces \"kube-oidc-proxy-e2e-sa-passthrough\" is forbidden: User \"test-username\" cannot get resource \"namespaces\" in API group \"\" in the namespace \"kube-oidc-proxy-e2e-sa-passthrough\"","reason":"Forbidden","details":{"name":"kube-oidc-proxy-e2e-sa-passthrough","kind":"namespaces"},"code":403}
	`)

	testsLookupDisabled := map[string]passthroughTest{
		"a bad token should return an error": {
			code: 401,
			body: unauthorizedBody,
			tokenGen: func() (string, error) {
				return "bad-token", nil
			},
		},

		"sending oidc token should be impersonated": {
			code: 403,
			body: forbiddenBody,
			tokenGen: func() (string, error) {
				token, err := e2eSuite.signToken(e2eSuite.validToken())
				if err != nil {
					return "", err
				}

				return token, nil
			},
		},

		"a good legacy token should passthrough": {
			code: 200,
			tokenGen: func() (string, error) {
				_, pc := serviceaccount.LegacyClaims(sav1, secv1)
				token, err := gener.GenerateToken(sc, pc)
				if err != nil {
					return "", err
				}

				return token, nil
			},
		},

		"a good scoped token should be rejected": {
			code: 401,
			body: unauthorizedBody,
			tokenGen: func() (string, error) {
				_, pc := serviceaccount.Claims(sa, pod, sec, 100, []string{"api"})
				token, err := gener.GenerateToken(sc, pc)
				if err != nil {
					return "", err
				}

				return token, nil
			},
		},
	}

	testsLookupEnabled := map[string]passthroughTest{
		"a bad token should return an error": {
			code: 401,
			body: unauthorizedBody,
			tokenGen: func() (string, error) {
				return "bad-token", nil
			},
		},

		"sending oidc token should be impersonated": {
			code: 403,
			body: forbiddenBody,
			tokenGen: func() (string, error) {
				return string(e2eSuite.validToken()), nil
			},
		},

		"a good legacy token should passthrough": {
			code: 200,
			tokenGen: func() (string, error) {
				_, pc := serviceaccount.LegacyClaims(sav1, secv1)
				token, err := gener.GenerateToken(sc, pc)
				if err != nil {
					return "", err
				}

				return token, nil
			},
		},

		"a good scoped token should passthrough": {
			code: 200,
			tokenGen: func() (string, error) {
				_, pc := serviceaccount.Claims(sa, pod, sec, 100, []string{"api"})
				token, err := gener.GenerateToken(sc, pc)
				if err != nil {
					return "", err
				}

				return token, nil
			},
		},
	}

	passthroughTry(t, false, testsLookupDisabled)

	// Start new proxy with sa passthrough enabled. Destroy at end of test
	e2eSuite.cleanup()

	err = e2eSuite.runProxy(
		"--service-account-token-passthrough",
		"--api-audiences=api",
		"--service-account-issuer=kubernetes/serviceaccount",
		"--service-account-key-file="+filepath.Join(e2eSuite.tmpDir, "sa.pub"),
		"--service-account-lookup=true",
		"--service-account-max-token-expiration=20m",
		"--v=10",
	)
	if err != nil {
		t.Errorf("failed to restart proxy: %s", err)
		t.FailNow()
	}

	defer e2eSuite.cleanup()

	passthroughTry(t, true, testsLookupEnabled)
}

func passthroughTry(t *testing.T, lookup bool, cases map[string]passthroughTest) {
	url := fmt.Sprintf(
		"https://127.0.0.1:%s/api/v1/namespaces/%s/",
		e2eSuite.proxyPort,
		namespaceSAPassthroughTest,
	)

	for n, c := range cases {
		token, err := c.tokenGen()
		if err != nil {
			t.Errorf("lookup=%t %s: unexpected error generating token: %s",
				lookup, n, err)
			continue
		}

		e2eSuite.wrappedRT.token = token
		resp, err := e2eSuite.proxyClient.Get(url)
		if err != nil {
			t.Errorf("lookup=%t %s: unexpected error: %s",
				lookup, n, err)
			continue
		}

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Errorf("lookup=%t %s: got unexpected read error: %s", lookup, n, err)
			continue
		}

		if c.body != nil && string(c.body) != strings.TrimRight(string(b), " ") {
			t.Errorf("lookup=%t %s: got unexpected body, exp=%s got=%s",
				lookup, n, c.body, b)
		}

		if c.code != resp.StatusCode {
			t.Errorf("lookup=%t %s: got unexpected status code, exp=%d got=%d\n%s\n",
				lookup, n, c.code, resp.StatusCode, b)
		}
	}
}
