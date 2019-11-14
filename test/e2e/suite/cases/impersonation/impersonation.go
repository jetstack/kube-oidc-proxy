package impersonation

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/jetstack/kube-oidc-proxy/test/e2e/framework"
)

var _ = framework.CasesDescribe("Impersonation", func() {
	f := framework.NewDefaultFramework("impersonation")

	It("should error at proxy when impersonation enabled and impersonation is attempted on a request", func() {
		By("Impersonating as a user")
		tryImpersonationClient(f, rest.ImpersonationConfig{
			UserName: "user@example.com",
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

	It("should not error at proxy when impersonation is disabled impersonation is attempted on a request", func() {
		f.DeployProxyWith("--disable-impersonation")

		// Should return a normal RBAC forbidden from Kubernetes. If is not a
		// Kubernetes error then it came from the kube-oidc-proxy so error
		client := f.NewProxyClient()
		_, err := client.CoreV1().Pods(f.Namespace.Name).List(metav1.ListOptions{})
		if !k8sErrors.IsForbidden(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	})
})

func tryImpersonationClient(f *framework.Framework, impConfig rest.ImpersonationConfig) {
	config := f.NewProxyRestConfig()
	config.Impersonate = impConfig

	tranConfig, err := config.TransportConfig()
	Expect(err).NotTo(HaveOccurred())

	client := http.DefaultClient
	client.Transport = tranConfig.Transport

	// send request with signed token to proxy
	target := fmt.Sprintf("%s/api/v1/namespaces/%s/pods",
		config.Host, f.Namespace.Name)

	resp, err := client.Get(target)
	Expect(err).NotTo(HaveOccurred())

	body, err := ioutil.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	expRespBody := []byte("Impersonation requests are disabled when using kube-oidc-proxy\n")

	// check body and status code the token was rejected
	if resp.StatusCode != http.StatusForbidden ||
		!bytes.Equal(body, expRespBody) {
	}
	Expect(fmt.Errorf("expected status code %d with body %q, got= %d %q",
		http.StatusForbidden, expRespBody, resp.StatusCode, body)).NotTo(HaveOccurred())
}
