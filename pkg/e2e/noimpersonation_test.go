// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespaceNoImpersonation = "kube-oidc-proxy-e2e-no-impersonation"
)

func TestNoImpersonation(t *testing.T) {
	if e2eSuite == nil {
		t.Skip("e2eSuite not defined")
		return
	}

	coreclient := e2eSuite.kubeclient.CoreV1()

	_, err := coreclient.Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceNoImpersonation,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

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

	// valid token
	e2eSuite.testToken(t, e2eSuite.validToken(), url, 200, "")
}
