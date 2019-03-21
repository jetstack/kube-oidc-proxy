// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jetstack/kube-oidc-proxy/pkg/e2e"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

const (
	defaultNodeImage = "v1.13.3"
)

func Test_E2E(t *testing.T) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "kube-oidc-proxy")
	if err != nil {
		t.Error(err)
		return
	}
	defer cleanup(t, tmpDir)

	nodeImage := os.Getenv("KUBE_OIDC_PROXY_NODE_IMAGE")
	if nodeImage == "" {
		nodeImage = defaultNodeImage
	}

	command := "../../bin/kind"
	args := []string{
		"create",
		"cluster",
		"--name=kube-oidc-proxy-e2e",
		fmt.Sprintf("--image=kindest/node:%s", nodeImage),
	}

	klog.Infof("running %s %s", command, args)
	cmd := exec.Command(command, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err = cmd.Run(); err != nil {
		t.Error(err)
		return
	}

	cmd = exec.Command("../../bin/kind", "get", "kubeconfig-path", "--name=kube-oidc-proxy-e2e")
	out, err := cmd.Output()
	if err != nil {
		t.Error(err)
		return
	}

	kubeconfig := strings.TrimSpace(string(out))

	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Error(err)
		return
	}

	e2eSuite := e2e.New(t, kubeconfig, tmpDir, restConfig)
	e2eSuite.Run()
}

func cleanup(t *testing.T, tmpDir string) {
	if err := os.RemoveAll(tmpDir); err != nil {
		t.Errorf("failed to delete temp dir %s: %s", tmpDir, err)
	}

	klog.Info("cleaning up kind cluster...")
	cmd := exec.Command("../../bin/kind", "delete", "cluster", "--name=kube-oidc-proxy-e2e")
	if err := cmd.Run(); err != nil {
		t.Errorf("failed to delete kind cluster: %s", err)
	}
}
