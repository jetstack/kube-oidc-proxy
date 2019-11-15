package upgrade

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"

	// required to register oidc auth plugin for rest client
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"github.com/jetstack/kube-oidc-proxy/pkg/util"
	"github.com/jetstack/kube-oidc-proxy/test/e2e/framework"
)

var _ = framework.CasesDescribe("Upgrade", func() {
	f := framework.NewDefaultFramework("upgrade")

	var pod *corev1.Pod
	JustBeforeEach(func() {
		By("Deploying echo server to exec to")

		var err error
		pod, err = f.Helper().KubeClient.CoreV1().Pods(f.Namespace.Name).Create(&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "echoserver-",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            "echoserver",
						Image:           "gcr.io/google_containers/echoserver:1.4",
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		err = f.Helper().WaitForPodReady(f.Namespace.Name, pod.Name, time.Second*30)
		Expect(err).NotTo(HaveOccurred())

		By("Creating Role")
		_, err = f.Helper().KubeClient.RbacV1().Roles(f.Namespace.Name).Create(&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name: "e2e-test-exec",
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
		Expect(err).NotTo(HaveOccurred())

		By("Creating RoleBinding")
		_, err = f.Helper().KubeClient.RbacV1().RoleBindings(f.Namespace.Name).Create(
			&rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "e2e-test-exec",
				},
				Subjects: []rbacv1.Subject{
					{
						Name: "user@example.com",
						Kind: "User",
					},
				},
				RoleRef: rbacv1.RoleRef{
					Name: "e2e-test-exec",
					Kind: "Role",
				},
			})
		Expect(err).NotTo(HaveOccurred())
	})

	It("should be able to exec into pod through the proxy", func() {
		restConfig := newRestConfig(f)

		restClient, err := rest.RESTClientFor(restConfig)
		Expect(err).NotTo(HaveOccurred())

		// curl echo server from within pod
		req := restClient.Post().
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
		Expect(err).NotTo(HaveOccurred())

		execOut := &bytes.Buffer{}
		execErr := &bytes.Buffer{}

		sopt := remotecommand.StreamOptions{
			Stdout: execOut,
			Stderr: execErr,
			Tty:    false,
		}

		By("Running exec into pod and runing curl on local host")
		err = exec.Stream(sopt)
		Expect(err).NotTo(HaveOccurred())

		// should have no stderr output
		if execErr.String() != "" {
			err := fmt.Errorf("got curl error: %s", execErr.String())
			Expect(err).NotTo(HaveOccurred())
		}

		By(fmt.Sprintf("exec outpu t%s/%s: %s", pod.Namespace, pod.Name, execOut.String()))

		// should have correct stdout output from echo server
		if !strings.HasSuffix(execOut.String(), "BODY:\nhello world") {
			err := fmt.Errorf("got unexpected echoserver response: exp=...hello world got=%s",
				execOut.String())
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("it should be able to sustain a port forward to send traffic", func() {
		By("Creating a port forward")

		portOut := &bytes.Buffer{}
		portErr := &bytes.Buffer{}

		freePort, err := util.FreePort()
		Expect(err).NotTo(HaveOccurred())

		restConfig := newRestConfig(f)

		restClient, err := rest.RESTClientFor(restConfig)
		Expect(err).NotTo(HaveOccurred())

		req := restClient.Post().
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

		errCh := make(chan error)

		go func() {
			defer close(pfopts.stopCh)

			// give a chance to establish a connection
			time.Sleep(time.Second * 2)

			By("Attempting to curl through port forward")

			portInR := bytes.NewReader([]byte("hello world"))

			// send message through port forward
			resp, err := http.Post(
				fmt.Sprintf("http://127.0.0.1:%s", freePort), "", portInR)
			if err != nil {
				errCh <- err
				return
			}

			// expect 200 resp and correct body
			if resp.StatusCode != 200 {
				errCh <- fmt.Errorf("got unexpected response code from server, exp=200 got=%d",
					resp.StatusCode)
				return
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				errCh <- fmt.Errorf("failed to read body: %s", err)
				return
			}

			// should have correct output from echo server
			if !bytes.HasSuffix(body, []byte("BODY:\nhello world")) {
				errCh <- fmt.Errorf("execOut.String())got unexpected echoserver response: exp=...hello world got=%s",
					body)
				return
			}
		}()

		By("Running port forward")
		if err := forwardPorts("POST", req.URL(), pfopts); err != nil {
			Expect(err).NotTo(HaveOccurred())
		}

		select {
		case <-pfopts.stopCh:
			return
		case err := <-errCh:
			Expect(err).NotTo(HaveOccurred())
		}
	})
})

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

func newRestConfig(f *framework.Framework) *rest.Config {
	payload := f.Helper().NewTokenPayload(f.IssuerURL(), f.ClientID(), time.Now().Add(time.Minute))
	signedToken, err := f.Helper().SignToken(f.IssuerKeyBundle(), payload)
	Expect(err).NotTo(HaveOccurred())

	return &rest.Config{
		Host: f.ProxyURL(),
		AuthProvider: &clientcmdapi.AuthProviderConfig{
			Name: "oidc",
			Config: map[string]string{
				"client-id":      f.ClientID(),
				"id-token":       signedToken,
				"idp-issuer-url": f.IssuerURL(),
			},
		},
		TLSClientConfig: rest.TLSClientConfig{
			CAData: f.ProxyKeyBundle().CertBytes,
		},

		APIPath: "/api",
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &corev1.SchemeGroupVersion,
			NegotiatedSerializer: scheme.Codecs,
		},
	}
}
