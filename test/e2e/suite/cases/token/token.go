package token

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jetstack/kube-oidc-proxy/test/e2e/framework"
)

type wraperRT struct {
	transport http.RoundTripper
	token     string
}

func (w *wraperRT) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Add("Authorization", fmt.Sprintf("bearer %s", w.token))
	return w.transport.RoundTrip(r)
}

var _ = framework.CasesDescribe("Token", func() {
	f := framework.NewDefaultFramework("token")

	It("should error when tokens are wrong for the issuer", func() {
		By("No token should error")
		expectProxyUnauthorized(f, nil)

		By("Bad token should error")
		expectProxyUnauthorized(f, []byte("bad token"))

		By("Wrong issuer should error")
		expectProxyUnauthorized(f, f.Helper().NewTokenPayload(
			"incorrect-issuer", f.ClientID(), time.Now().Add(time.Minute)))

		By("Wrong audience should error")
		expectProxyUnauthorized(f, f.Helper().NewTokenPayload(
			f.IssuerURL(), "wrong-aud", time.Now().Add(time.Minute)))

		By("Token expires now")
		expectProxyUnauthorized(f, f.Helper().NewTokenPayload(
			f.IssuerURL(), f.ClientID(), time.Now()))

		By("Valid token should return Kubernetes forbidden")
		client := f.NewProxyClient()

		// if does not return with Kubernetes forbidden error then error
		_, err := client.CoreV1().Pods(f.Namespace.Name).List(metav1.ListOptions{})
		if !k8sErrors.IsForbidden(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	})
})

func expectProxyUnauthorized(f *framework.Framework, tokenPayload []byte) {
	// build client using given token payload
	signedToken, err := f.Helper().SignToken(f.IssuerKeyBundle(), tokenPayload)
	Expect(err).NotTo(HaveOccurred())

	proxyConfig := f.NewProxyRestConfig()
	client := http.DefaultClient
	client.Transport = &wraperRT{
		transport: proxyConfig.Transport,
		token:     signedToken,
	}

	// send request with signed token to proxy
	target := fmt.Sprintf("%s/api/v1/namespaces/%s/pods",
		proxyConfig.Host, f.Namespace.Name)

	resp, err := client.Get(target)
	Expect(err).NotTo(HaveOccurred())

	body, err := ioutil.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	// check body and status code the token was rejected
	if resp.StatusCode != http.StatusForbidden ||
		!bytes.Equal(body, []byte("Unauthorized")) {
	}
	Expect(fmt.Errorf("expected status code %d with body Unauthorized, got= %d %q",
		http.StatusForbidden, resp.StatusCode, body)).NotTo(HaveOccurred())
}
