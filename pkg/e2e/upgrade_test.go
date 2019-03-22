// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/kubernetes/pkg/kubectl/cmd/exec"
	cmdportforward "k8s.io/kubernetes/pkg/kubectl/cmd/portforward"

	"github.com/jetstack/kube-oidc-proxy/pkg/utils"
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
				Resources: []string{
					"pods", "pods/exec", "pods/portforward",
				},
				Verbs: []string{
					"get", "list", "create",
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

	// rest config pointed to proxy
	restConfig, err := e2eSuite.proxyRestClient()
	if err != nil {
		t.Fatal(err)
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

		if i == 30 {
			t.Fatalf("echo server failed to become ready: %s",
				pod.Status.Phase)
		}

		time.Sleep(time.Second * 5)
		i++
	}

	var execOut bytes.Buffer
	var execErr bytes.Buffer

	// curl echo server from within pod
	ioStreams := genericclioptions.IOStreams{In: nil, Out: &execOut, ErrOut: &execErr}
	execOptions := &exec.ExecOptions{
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

	if err := execOptions.Validate(); err != nil {
		t.Fatal(err)
	}

	if err := execOptions.Run(); err != nil {
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

	// test we can port forward to the echo server and curl on localhost to it

	var portOut bytes.Buffer
	var portErr bytes.Buffer

	freePort, err := utils.FreePort()
	if err != nil {
		t.Fatal(err)
	}

	RESTClient, err := rest.RESTClientFor(restConfig)
	if err != nil {
		t.Fatal(err)
	}

	ioStreams = genericclioptions.IOStreams{In: nil, Out: &portOut, ErrOut: &portErr}

	portForwardOptions := &cmdportforward.PortForwardOptions{
		Namespace:  pod.Namespace,
		PodName:    pod.Name,
		RESTClient: RESTClient,
		Config:     restConfig,
		PodClient:  kubeclient.Core(),
		Address:    []string{"127.0.0.1"},
		Ports:      []string{freePort + ":8080"},
		PortForwarder: &defaultPortForwarder{
			IOStreams: ioStreams,
		},
		StopChannel:  make(chan struct{}, 1),
		ReadyChannel: make(chan struct{}),
	}

	go func() {
		defer close(portForwardOptions.StopChannel)

		// give a chance to establish a connection
		time.Sleep(time.Second * 2)

		portInR := bytes.NewReader([]byte("hello world"))

		resp, err := http.Post(
			fmt.Sprintf("http://127.0.0.1:%s", freePort), "", portInR)
		if err != nil {
			t.Errorf("failed to request echoserver from port forward: %s", err)
			return
		}

		if resp.StatusCode != 200 {
			t.Errorf("got unexpected response code from server, exp=200 got=%d",
				resp.StatusCode)
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Errorf("failed to read body: %s", err)
			return
		}

		// should have correct output from echo server
		if !bytes.HasSuffix(body, []byte("BODY:\nhello world")) {
			t.Errorf("got unexpected echoserver response: exp=...hello world got=%s",
				execOut.String())
		}
	}()

	if err := portForwardOptions.Validate(); err != nil {
		t.Fatal(err)
	}

	if err := portForwardOptions.RunPortForward(); err != nil {
		t.Fatal(err)
	}
}

/////// taken from k8s.io/kubernetes/pkg/kubectl/cmd/portforward
type defaultPortForwarder struct {
	genericclioptions.IOStreams
}

func (f *defaultPortForwarder) ForwardPorts(method string, url *url.URL, opts cmdportforward.PortForwardOptions) error {
	transport, upgrader, err := spdy.RoundTripperFor(opts.Config)
	if err != nil {
		return err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, method, url)
	fw, err := portforward.NewOnAddresses(dialer, opts.Address, opts.Ports, opts.StopChannel, opts.ReadyChannel, f.Out, f.ErrOut)
	if err != nil {
		return err
	}
	return fw.ForwardPorts()
}

///////
