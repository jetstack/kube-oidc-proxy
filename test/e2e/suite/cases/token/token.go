// Copyright Jetstack Ltd. See LICENSE for details.
package token

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jetstack/kube-oidc-proxy/test/e2e/framework"
)

var _ = framework.CasesDescribe("Token", func() {
	f := framework.NewDefaultFramework("token")

	It("should error when tokens are wrong for the issuer", func() {
		By("No token should error")
		expectProxyUnauthorized(f, nil)

		By("Bad token should error")
		expectProxyUnauthorized(f, []byte("bad token"))

		By("Wrong issuer should error")
		badURL, err := url.Parse("incorrect-issuer.io")
		Expect(err).NotTo(HaveOccurred())

		expectProxyUnauthorized(f, f.Helper().NewTokenPayload(
			badURL, f.ClientID(), time.Now().Add(time.Minute)))

		By("Wrong audience should error")
		expectProxyUnauthorized(f, f.Helper().NewTokenPayload(
			f.IssuerURL(), "wrong-aud", time.Now().Add(time.Minute)))

		By("Token expires now")
		expectProxyUnauthorized(f, f.Helper().NewTokenPayload(
			f.IssuerURL(), f.ClientID(), time.Now()))

		By("Valid token should return Kubernetes forbidden")
		client := f.NewProxyClient()

		// If does not return with Kubernetes forbidden error then error
		_, err = client.CoreV1().Pods(f.Namespace.Name).List(context.Background(), metav1.ListOptions{})
		if !k8sErrors.IsForbidden(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	})
})

func expectProxyUnauthorized(f *framework.Framework, tokenPayload []byte) {
	// Build client using given token payload
	signedToken, err := f.Helper().SignToken(f.IssuerKeyBundle(), tokenPayload)
	Expect(err).NotTo(HaveOccurred())

	proxyConfig := f.NewProxyRestConfig()
	requester := f.Helper().NewRequester(proxyConfig.Transport, signedToken)

	// Send request with signed token to proxy
	target := fmt.Sprintf("%s/api/v1/namespaces/%s/pods",
		proxyConfig.Host, f.Namespace.Name)

	body, resp, err := requester.Get(target)
	body = bytes.TrimSpace(body)
	Expect(err).NotTo(HaveOccurred())

	// Check body and status code the token was rejected
	if resp.StatusCode != http.StatusUnauthorized ||
		!bytes.Equal(body, []byte("Unauthorized")) {
		Expect(fmt.Errorf("expected status code %d with body Unauthorized, got= %d %q",
			http.StatusUnauthorized, resp.StatusCode, body)).NotTo(HaveOccurred())
	}
}
