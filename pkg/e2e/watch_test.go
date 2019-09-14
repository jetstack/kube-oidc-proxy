// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/jetstack/kube-oidc-proxy/pkg/util"
)

func Test_WatchSecretFiles(t *testing.T) {
	if e2eSuite == nil {
		t.Skip("e2eSuite not defined")
		return
	}

	readinessPort, err := util.FreePort()
	if err != nil {
		t.Fatal(err)
	}

	pairTmpDir, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(pairTmpDir); err != nil {
			t.Error(err)
		}
	}()

	keyCertPair, err := util.NewTLSSelfSignedCertKey(pairTmpDir, "")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	err = e2eSuite.runProxy(
		"--readiness-probe-port="+readinessPort,
		"--reload-watch-refresh-period=5s",
		fmt.Sprintf("--reload-watch-files=%s,%s",
			keyCertPair.CertPath, keyCertPair.KeyPath),
		"--tls-cert-file="+keyCertPair.CertPath,
		"--tls-private-key-file="+keyCertPair.KeyPath,
	)

	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	defer e2eSuite.cleanup()

	if err := ioutil.WriteFile(keyCertPair.CertPath, []byte("aa"), 0600); err != nil {
		t.Error(err)
		return
	}

	cmd := e2eSuite.proxyCmd

	waitCh := make(chan error)
	go func() {
		waitCh <- cmd.Wait()
	}()

	waitTime := time.Second * 10
	timer := time.NewTimer(waitTime)

	// wait for process to exit or until timer ticks
	select {
	case err := <-waitCh:
		if err != nil {
			t.Errorf("error waiting for process to complete: %s",
				err)
		}

	case <-timer.C:
		t.Errorf("process did not exit in expected time frame (%s)",
			waitTime.String())
	}

	if cmd.ProcessState == nil {
		t.Errorf("unexpected process state, got=%v",
			cmd.ProcessState)
	} else if !cmd.ProcessState.Exited() {
		t.Errorf(
			"expected kube-oidc-proxy to have exited after file change, but it's still running: %s",
			cmd.ProcessState.String())
	} else {
		t.Log("process exited as expected")
	}
}
