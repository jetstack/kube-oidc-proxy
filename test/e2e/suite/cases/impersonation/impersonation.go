// Copyright Jetstack Ltd. See LICENSE for details.
package impersonation

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/jetstack/kube-oidc-proxy/test/e2e/framework"
)

var _ = framework.CasesDescribe("Impersonation", func() {
	f := framework.NewDefaultFramework("impersonation")

	It("should error at proxy when impersonation enabled and impersonation is attempted on a request", func() {
		By("Impersonating as a user")
		tryImpersonationClient(f, rest.ImpersonationConfig{
			UserName: "foo@example.com",
		})

		By("Impersonating as a group")
		tryImpersonationClient(f, rest.ImpersonationConfig{
			Groups: []string{
				"group-1",
				"group-2",
			},
		})

		By("Impersonating as a extra")
		tryImpersonationClient(f, rest.ImpersonationConfig{
			Extra: map[string][]string{
				"foo": {
					"k1", "k2", "k3",
				},
				"bar": {
					"k1", "k2", "k3",
				},
			},
		})

		By("Impersonating as a user, group and extra")
		tryImpersonationClient(f, rest.ImpersonationConfig{
			UserName: "user@example.com",
			Groups: []string{
				"group-1",
				"group-2",
			},
			Extra: map[string][]string{
				"foo": {
					"k1", "k2", "k3",
				},
				"bar": {
					"k1", "k2", "k3",
				},
			},
		})
	})

	It("should not error at proxy when impersonation is disabled and impersonation is attempted on a request", func() {
		By("Enabling the disabling of impersonation")
		err := f.DeployProxyWith([]string{"--disable-impersonation"})
		Expect(err).NotTo(HaveOccurred())

		// Should return an Unauthorized response from Kubernetes as it does not
		// trust the OIDC token we have presented however it has been authenticated
		// by kube-oidc-proxy.
		_, err = f.NewProxyClient().CoreV1().Pods(f.Namespace.Name).List(metav1.ListOptions{})
		if !k8sErrors.IsUnauthorized(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	})
})

func tryImpersonationClient(f *framework.Framework, impConfig rest.ImpersonationConfig) {
	// build client with impersonation
	config := f.NewProxyRestConfig()
	config.Impersonate = impConfig
	client, err := kubernetes.NewForConfig(config)
	Expect(err).NotTo(HaveOccurred())

	_, err = client.CoreV1().Pods(f.Namespace.Name).List(metav1.ListOptions{})
	kErr, ok := err.(*k8sErrors.StatusError)
	if !ok {
		Expect(err).NotTo(HaveOccurred())
	}

	expRespBody := "Impersonation requests are disabled when using kube-oidc-proxy"
	resp := kErr.Status().Details.Causes[0].Message

	// check body and status code the token was rejected
	if int(kErr.Status().Code) != http.StatusForbidden ||
		resp != expRespBody {
		Expect(fmt.Errorf("expected status code %d with body %q, got=%s",
			http.StatusForbidden, expRespBody, kErr)).NotTo(HaveOccurred())
	}
}
