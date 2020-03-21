// Copyright Jetstack Ltd. See LICENSE for details.
package probe

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/jetstack/kube-oidc-proxy/test/e2e/framework"
	"github.com/jetstack/kube-oidc-proxy/test/kind"
)

var _ = framework.CasesDescribe("Readiness Probe", func() {
	f := framework.NewDefaultFramework("readiness-probe")

	It("Should not become ready if the issuer is unavailable", func() {
		By("Deleting the Issuer so no longer becomes reachable")
		Expect(f.Helper().DeleteIssuer(f.Namespace.Name)).NotTo(HaveOccurred())

		By("Deleting the current proxy that is ready")
		Expect(f.Helper().DeleteProxy(f.Namespace.Name)).NotTo(HaveOccurred())

		By("Deploying the Proxy should never become ready as the issuer is unavailable")
		_, _, err := f.Helper().DeployProxy(f.Namespace, f.IssuerURL(),
			f.ClientID(), f.IssuerKeyBundle(), nil)
		// Error should occur (not ready)
		Expect(err).To(HaveOccurred())
	})

	It("Should continue to be ready even if the issuer becomes unavailable", func() {
		By("Deleting the Issuer so no longer becomes reachable")
		Expect(f.Helper().DeleteIssuer(f.Namespace.Name)).NotTo(HaveOccurred())

		time.Sleep(time.Second * 10)

		err := f.Helper().WaitForDeploymentReady(f.Namespace.Name, kind.ProxyImageName, time.Second*5)
		Expect(err).NotTo(HaveOccurred())
	})
})
