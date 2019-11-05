// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"fmt"
	"io/ioutil"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespaceTokenPassthroughTest = "kube-oidc-proxy-e2e-token-passthrough"
)

func Test_TokenPassthrough(t *testing.T) {
	mustSkipMissingSuite(t)
	mustNamespace(t, namespaceTokenPassthroughTest)
	mustCreatePodRbac(t, "test-username", namespaceTokenPassthroughTest, "User")
	mustCreatePodRbac(t, "test-service-account", namespaceTokenPassthroughTest, "ServiceAccount")

	defer e2eSuite.cleanup()

	testServiceAccountName := "test-service-account"
	coreClient := e2eSuite.kubeclient.CoreV1()

	sa, err := coreClient.ServiceAccounts(namespaceTokenPassthroughTest).Create(&corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testServiceAccountName,
			Namespace: namespaceTokenPassthroughTest,
		},
	})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	sec := waitForSASecret(t, sa.Name, sa.Namespace)

	saToken, ok := sec.Data[corev1.ServiceAccountTokenKey]
	if !ok {
		t.Errorf("expected token to be present in secret %s/%s: %+v",
			sec.Name, sec.Namespace, sec.Data)
		t.FailNow()
	}

	type passthroughT struct {
		token   func() (string, error)
		expCode int
	}

	url := fmt.Sprintf(
		"https://127.0.0.1:%s/api/v1/namespaces/%s/pods",
		e2eSuite.proxyPort,
		namespaceTokenPassthroughTest,
	)

	runTest := func(t *testing.T, test passthroughT) {
		token, err := test.token()
		if err != nil {
			t.Errorf("unexpected error generating token: %s", err)
			t.FailNow()
		}

		e2eSuite.wrappedRT.token = token
		resp, err := e2eSuite.proxyClient.Get(url)
		if err != nil {
			t.Errorf("unexpected error: %s", err)
			t.FailNow()
		}

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Errorf("got unexpected read error: %s", err)
			t.FailNow()
		}

		t.Logf("got response: %s", b)

		if test.expCode != resp.StatusCode {
			t.Errorf("got unexpected status code, exp=%d got=%d\n%s\n",
				test.expCode, resp.StatusCode, b)
			t.Errorf("got response: %s", b)
		}
	}

	noPassthroughTests := map[string]passthroughT{
		"no pasthrough: a valid oidc token should always forward as normal": {
			token: func() (string, error) {
				return e2eSuite.signToken(e2eSuite.validToken())
			},
			expCode: 200,
		},

		"no passthrough: a service account token should be rejected": {
			token: func() (string, error) {
				return string(saToken), nil
			},
			expCode: 401,
		},
	}

	passthroughTests := map[string]passthroughT{
		"passthrough: a valid oidc token should forward as normal": {
			token: func() (string, error) {
				return e2eSuite.signToken(e2eSuite.validToken())
			},
			expCode: 200,
		},

		"passthrough: a service account token should be passed through": {
			token: func() (string, error) {
				return string(saToken), nil
			},
			expCode: 200,
		},
	}

	for _, tt := range []map[string]passthroughT{
		noPassthroughTests, passthroughTests,
	} {
		for name, test := range tt {
			t.Run(name, func(t *testing.T) {
				runTest(t, test)
			})
		}
		e2eSuite.cleanup()
		if err := e2eSuite.runProxy("--token-passthrough"); err != nil {
			t.Error(err)
			t.FailNow()
		}
	}
}
