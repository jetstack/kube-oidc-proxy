// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

const (
	defaultNodeImage = "v1.14.0"
)

var e2eSuite *E2E

func TestMain(m *testing.M) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "kube-oidc-proxy")
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}

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
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}

	cmd = exec.Command("../../bin/kind", "get", "kubeconfig-path", "--name=kube-oidc-proxy-e2e")
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		cleanup(tmpDir, 1)
	}

	kubeconfig := strings.TrimSpace(string(out))

	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		cleanup(tmpDir, 1)
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		cleanup(tmpDir, 1)
	}

	e2eSuite = New(kubeconfig, tmpDir, kubeClient)
	e2eSuite.Run()

	runErr := m.Run()

	e2eSuite.cleanup()
	cleanup(tmpDir, runErr)
}

func cleanup(tmpDir string, exitCode int) {
	err := os.RemoveAll(tmpDir)
	if err != nil {
		klog.Errorf("failed to delete temp dir %s: %s", tmpDir, err)
	}

	klog.Info("cleaning up kind cluster...")
	cmd := exec.Command("../../bin/kind", "delete", "cluster", "--name=kube-oidc-proxy-e2e")
	cmdErr := cmd.Run()
	if cmdErr != nil {
		klog.Errorf("failed to delete kind cluster: %s", cmdErr)
	}

	if err != nil || cmdErr != nil {
		os.Exit(1)
	}

	os.Exit(exitCode)
}
