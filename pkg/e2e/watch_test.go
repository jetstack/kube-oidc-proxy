// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/jetstack/kube-oidc-proxy/pkg/utils"
)

func Test_WatchSecretFiles(t *testing.T) {
	proxyPort, err := utils.FreePort()
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

	proxyCertPath, proxyKeyPath, err := utils.NewTLSSelfSignedCertKey(pairTmpDir, "")
	if err != nil {
		t.Error(err)
		return
	}

	cmd := exec.Command("../../kube-oidc-proxy",
		"--oidc-issuer-url=https://127.0.0.1:1234",
		"--oidc-client-id=kube-oidc-proy_e2e_client-id",
		"--oidc-username-claim=e2e-username-claim",
		"--secret-watch-refresh-period=5",

		"--bind-address=127.0.0.1",
		fmt.Sprintf("--secure-port=%s", proxyPort),
		fmt.Sprintf("--tls-cert-file=%s", proxyCertPath),
		fmt.Sprintf("--tls-private-key-file=%s", proxyKeyPath),

		"--v=10",
	)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	defer func() {
		// process already exited
		if cmd.ProcessState != nil &&
			cmd.ProcessState.Exited() {
			return
		}

		if cmd.Process == nil {
			t.Fatalf("failed to kill process, was nil: %v", cmd.Process)
		}

		if err := cmd.Process.Kill(); err != nil {
			t.Errorf("failed to kill kube-oidc-proxy process: %s", err)
		}
	}()

	time.Sleep(time.Second * 10)

	if err := ioutil.WriteFile(proxyCertPath, []byte("aa"), 0600); err != nil {
		t.Error(err)
		return
	}

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
