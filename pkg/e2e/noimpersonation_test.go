// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"fmt"
	"testing"
)

const (
	namespaceNoImpersonation = "kube-oidc-proxy-e2e-no-impersonation"
)

func TestNoImpersonation(t *testing.T) {
	mustSkipMissingSuite(t)
	mustNamespace(t, namespaceNoImpersonation)

	e2eSuite.cleanup()
	defer e2eSuite.cleanup()

	if err := e2eSuite.runProxy("--disable-impersonation"); err != nil {
		t.Error(err)
		t.FailNow()
	}

	url := fmt.Sprintf(
		"https://127.0.0.1:%s/api/v1/namespaces/%s/pods",
		e2eSuite.proxyPort,
		namespaceNoImpersonation,
	)

	// Should return an unathorized response from the API server - we authed with
	// the proxy but the API server isn't set up for our OIDC auth
	e2eSuite.testToken(t, e2eSuite.validToken(), url, 401,
		`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Failure","message":"Unauthorized","reason":"Unauthorized","code":401}`)
}
