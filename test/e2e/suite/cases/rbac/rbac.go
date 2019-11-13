package rbac

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	//. "github.com/onsi/gomega"

	"github.com/jetstack/kube-oidc-proxy/test/e2e/framework"
)

var _ = framework.CasesDescribe("RBAC", func() {
	f := framework.NewDefaultFramework("rbac")

	It("should do nothing", func() {
		fmt.Printf("%s\n", f.BaseName)
	})
})
