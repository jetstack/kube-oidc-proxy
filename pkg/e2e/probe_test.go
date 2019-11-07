// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespaceProbe = "kube-oidc-proxy-e2e-probe"
)

func TestProbe(t *testing.T) {
	mustSkipMissingSuite(t)
	mustNamespace(t, namespaceProbe)

	e2eSuite.cleanup()
	defer e2eSuite.cleanup()

	if err := e2eSuite.runProxy(); err != nil {
		t.Error(err)
		t.FailNow()
	}

}
