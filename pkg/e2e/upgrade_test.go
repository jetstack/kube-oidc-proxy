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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"

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

	// create upgrade namespace
	_, err := e2eSuite.kubeclient.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceUpgradeTest,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// create auth for user in namespace
	_, err = e2eSuite.kubeclient.RbacV1().Roles(namespaceUpgradeTest).Create(&rbacv1.Role{
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

	_, err = e2eSuite.kubeclient.RbacV1().RoleBindings(namespaceUpgradeTest).Create(
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
	pod, err := kubeclient.CoreV1().Pods(namespaceUpgradeTest).Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "echoserver",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "echoserver",
					Image:           "gcr.io/google_containers/echoserver:1.4",
					ImagePullPolicy: corev1.PullAlways,
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// wait for our echo server to become ready
	err = utils.WaitForPodReady(e2eSuite.kubeclient,
		"echoserver", namespaceUpgradeTest)
	if err != nil {
		t.Fatal(err)
	}

	RESTClient, err := rest.RESTClientFor(restConfig)
	if err != nil {
		t.Fatal(err)
	}

	// curl echo server from within pod
	req := RESTClient.Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "echoserver",
			Command: []string{
				"curl", "127.0.0.1:8080", "-s", "-d", "hello world",
			},
			Stdin:  false,
			Stdout: true,
			Stderr: true,
			TTY:    false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		t.Fatalf("failed to create SPDY executor: %s", err)
	}
	execOut := &bytes.Buffer{}
	execErr := &bytes.Buffer{}

	sopt := remotecommand.StreamOptions{
		Stdout: execOut,
		Stderr: execErr,
		Tty:    false,
	}

	err = exec.Stream(sopt)
	if err != nil {
		t.Fatalf("failed to execute stream command: %s", err)
	}

	// should have no stderr output
	if execErr.String() != "" {
		t.Errorf("got curl error: %s", execErr.String())
	}

	t.Logf("%s/%s: %s", pod.Namespace, pod.Name, execOut.String())

	// should have correct stdout output from echo server
	if !strings.HasSuffix(execOut.String(), "BODY:\nhello world") {
		t.Errorf("got unexpected echoserver response: exp=...hello world got=%s",
			execOut.String())
	}

	// test we can port forward to the echo server and curl on localhost to it
	portOut := &bytes.Buffer{}
	portErr := &bytes.Buffer{}

	freePort, err := utils.FreePort()
	if err != nil {
		t.Fatal(err)
	}

	req = RESTClient.Post().
		Resource("pods").
		Namespace(pod.Namespace).
		Name(pod.Name).
		SubResource("portforward")

	pfopts := &portForwardOptions{
		address:    []string{"127.0.0.1"},
		ports:      []string{freePort + ":8080"},
		stopCh:     make(chan struct{}, 1),
		readyCh:    make(chan struct{}),
		outBuf:     portOut,
		errBuf:     portErr,
		restConfig: restConfig,
	}

	go func() {
		defer close(pfopts.stopCh)

		// give a chance to establish a connection
		time.Sleep(time.Second * 2)

		portInR := bytes.NewReader([]byte("hello world"))

		// send message through port forward
		resp, err := http.Post(
			fmt.Sprintf("http://127.0.0.1:%s", freePort), "", portInR)
		if err != nil {
			t.Errorf("failed to request echoserver from port forward: %s", err)
			return
		}

		// expect 200 resp and correct body
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

	if err := forwardPorts("POST", req.URL(), pfopts); err != nil {
		t.Error(err)
		return
	}

}

type portForwardOptions struct {
	address, ports  []string
	readyCh, stopCh chan struct{}
	outBuf, errBuf  *bytes.Buffer
	restConfig      *rest.Config
}

func forwardPorts(method string, url *url.URL, opts *portForwardOptions) error {
	transport, upgrader, err := spdy.RoundTripperFor(opts.restConfig)
	if err != nil {
		return err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, method, url)
	fw, err := portforward.NewOnAddresses(dialer, opts.address,
		opts.ports, opts.stopCh, opts.readyCh, opts.outBuf, opts.errBuf)
	if err != nil {
		return err
	}

	return fw.ForwardPorts()
}
