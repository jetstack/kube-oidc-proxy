// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strconv"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	"github.com/jetstack/kube-oidc-proxy/pkg/e2e/issuer"
)

func Test_E2E(t *testing.T) {
	// !!use kind instead of minikube

	tmpDir, err := ioutil.TempDir(os.TempDir(), "kube-oidc-proxy")
	if err != nil {
		t.Error(err)
		return
	}
	defer cleanup(t, tmpDir)

	freePort, err := freePort()
	if err != nil {
		t.Error(err)
		return
	}

	issuer := issuer.New(tmpDir, freePort)
	if err := issuer.Run(); err != nil {
		t.Error(err)
		return
	}

	if err := runCmd(t, "./bin/kind", "create", "cluster", "--name=kube-oidc-proxy-e2e"); err != nil {
		return
	}

	cmd := exec.Command("kind", "get", "kubeconfig-path", "--name=kube-oidc-proxy-e2e")
	b, err := cmd.Output()
	if err != nil {
		t.Error(err)
		return
	}
	//apiserverAddr := fmt.Sprintf("https://%s:8443", fields[len(fields)-1])
	//certFile := filepath.Join(os.Getenv("HOME"), ".minikube/client.crt")
	//keyFile := filepath.Join(os.Getenv("HOME"), ".minikube/client.key")
	//caFile := filepath.Join(os.Getenv("HOME"), ".minikube/ca.crt")

	restConfig, err := clientcmd.BuildConfigFromFlags("", string(b))
	if err != nil {
		t.Error(err)
		return
	}

	//clientConfigFlags := genericclioptions.NewConfigFlags()
	//clientConfigFlags.CertFile = &certFile
	//clientConfigFlags.KeyFile = &keyFile
	//clientConfigFlags.CAFile = &caFile
	//clientConfigFlags.APIServer = &apiserverAddr

	//restConfig, err := clientConfigFlags.ToRESTConfig()
	//if err != nil {
	//	t.Error(err)
	//	return
	//}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		t.Error(err)
		return
	}

	n, err := kubeClient.Core().Nodes().List(metav1.ListOptions{})
	klog.Infof("%+v\n", n)
	klog.Infof("%+v\n", err)

	//if err := runCmd(t, "minikube", "start"); err != nil {
	//	os.Exit(1)
	//}
}

func runCmd(t *testing.T, command string, args ...string) error {
	klog.Infof("running %s %s", command, args)
	cmd := exec.Command(command, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		t.Errorf("failed to run %s %s: %s", command, args, err)
		return err
	}

	return nil
}

func cleanup(t *testing.T, tmpDir string) {
	if err := os.RemoveAll(tmpDir); err != nil {
		t.Errorf("failed to delete temp dir %s: %s", tmpDir, err)
	}

	klog.Info("cleaning up kind cluster...")
	cmd := exec.Command("./bin/kind", "delete", "cluster", "--name=kube-oidc-proxy-e2e")
	if err := cmd.Run(); err != nil {
		t.Errorf("failed to delete kind cluster: %s", err)
	}
}

func freePort() (string, error) {
	l, err := net.ListenTCP("tcp", &net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0,
	})
	if err != nil {
		return "", err
	}
	defer l.Close()

	port := l.Addr().(*net.TCPAddr).Port
	return strconv.Itoa(port), nil
}
