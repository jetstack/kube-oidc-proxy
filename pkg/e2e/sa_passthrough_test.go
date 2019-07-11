// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import "testing"

func Test_Rbac(t *testing.T) {
	e2eSuite.skipNotReady(t)
	defer e2eSuite.cleanup()

	err := e2eSuite.runProxy("--service-account-token-passthrough")
	if err != nil {
		t.Errorf("failed to run proxy with sa token passthrough: %s", err)
		t.FailNow()
	}

}
