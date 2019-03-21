// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubernetes/pkg/kubectl/cmd/exec"
)

const (
	namespaceUpgradeTest = "kube-oidc-proxy-e2e-upgrade"
)

func Test_Upgrade(t *testing.T) {
	if e2eSuite == nil {
		t.Skip("e2eSuite not defined")
		return
	}

	defer func() {
		err := e2eSuite.kubeclient.Core().Namespaces().Delete(namespaceUpgradeTest, nil)
		if err != nil {
			t.Errorf("failed to delete test namespace: %s", err)
		}
	}()

	// create upgrade namespace
	_, err := e2eSuite.kubeclient.Core().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceUpgradeTest,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// create auth for user in namespace
	_, err = e2eSuite.kubeclient.Rbac().Roles(namespaceUpgradeTest).Create(&rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-username-role",
			Namespace: namespaceUpgradeTest,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "pods/logs"},
				Verbs: []string{
					"get", "list", "create",
				},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods/exec"},
				Verbs: []string{
					"create",
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = e2eSuite.kubeclient.Rbac().RoleBindings(namespaceUpgradeTest).Create(
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-username-binding",
				Namespace: namespaceUpgradeTest,
			},
			Subjects: []rbacv1.Subject{
				{
					Name: "test-username",
					Kind: "User",
				},
			},
			RoleRef: rbacv1.RoleRef{
				Name: "test-username-role",
				Kind: "Role",
			},
		})
	if err != nil {
		t.Fatal(err)
	}

	// valid signed token for auth to proxy
	signedToken, err := e2eSuite.signToken(e2eSuite.validToken())
	if err != nil {
		t.Fatal(err)
		return
	}

	// get rest config pointed to proxy
	restConfig := &rest.Config{
		Host: fmt.Sprintf("https://127.0.0.1:%s", e2eSuite.proxyPort),
		AuthProvider: &clientcmdapi.AuthProviderConfig{
			Name: "oidc",
			Config: map[string]string{
				"client-id":      "kube-oidc-proxy_e2e_client-id",
				"id-token":       signedToken,
				"idp-issuer-url": "https://127.0.0.1:" + e2eSuite.proxyPort,
			},
		},
		TLSClientConfig: rest.TLSClientConfig{
			CAData: e2eSuite.proxyCert,
		},

		APIPath: "/api",
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &corev1.SchemeGroupVersion,
			NegotiatedSerializer: scheme.Codecs,
		},
	}

	// get kubeclient pointed to proxy
	kubeclient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		t.Fatal(err)
	}

	// deploy echo server
	pod, err := kubeclient.Core().Pods(namespaceUpgradeTest).Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "echoserver",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "echoserver",
					Image: "gcr.io/google_containers/echoserver:1.4",
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// wait for pod to be ready
	i := 0
	for {
		pod, err = kubeclient.Core().Pods(namespaceUpgradeTest).Get("echoserver", metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}

		if pod.Status.Phase == corev1.PodRunning {
			break
		}

		if i == 10 {
			t.Fatalf("echo server failed to become ready: %s",
				pod.Status.Phase)
		}

		time.Sleep(time.Second * 5)
		i++
	}

	var (
		execOut bytes.Buffer
		execErr bytes.Buffer
	)

	// curl echo server from within pod
	ioStreams := genericclioptions.IOStreams{In: nil, Out: &execOut, ErrOut: &execErr}
	options := &exec.ExecOptions{
		StreamOptions: exec.StreamOptions{
			IOStreams: ioStreams,
			PodName:   pod.Name,
			Namespace: pod.Namespace,
		},
		Command: []string{
			"curl", "127.0.0.1:8080", "-s", "-d", "hello world",
		},
		PodClient: kubeclient.Core(),
		Config:    restConfig,
		Executor:  &exec.DefaultRemoteExecutor{},
	}

	err = options.Run()
	if err != nil {
		t.Fatal(err)
	}

	// should have no stderr output
	if execErr.String() != "" {
		t.Errorf("got curl error: %s", execErr.String())
	}

	// should have correct stdout output from echo server
	if !strings.HasSuffix(execOut.String(), "BODY:\nhello world") {
		t.Errorf("got unexpected echoserver response: exp=...hello world got=%s",
			execOut.String())
	}
}
