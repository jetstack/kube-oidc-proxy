// Copyright Jetstack Ltd. See LICENSE for details.
package suite

import (
	//"fmt"
	//"path"
	"testing"
	"time"

	"github.com/onsi/ginkgo"
	ginkgoconfig "github.com/onsi/ginkgo/config"

	//"github.com/onsi/ginkgo/reporters"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"

	_ "github.com/jetstack/kube-oidc-proxy/test/e2e/suite/cases"
)

func init() {
	// Turn on verbose by default to get spec names
	ginkgoconfig.DefaultReporterConfig.Verbose = true
	// Turn on EmitSpecProgress to get spec progress (especially on interrupt)
	ginkgoconfig.GinkgoConfig.EmitSpecProgress = true
	// Randomize specs as well as suites
	ginkgoconfig.GinkgoConfig.RandomizeAllSpecs = true

	wait.ForeverTestTimeout = time.Second * 60
}

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)

	var r []ginkgo.Reporter
	//if framework.DefaultConfig.Ginkgo.ReportDirectory != "" {
	//	r = append(r, reporters.NewJUnitReporter(path.Join(framework.DefaultConfig.Ginkgo.ReportDirectory,
	//		fmt.Sprintf("junit_%s_%02d.xml",
	//			framework.DefaultConfig.Ginkgo.ReportPrefix,
	//			ginkgoconfig.GinkgoConfig.ParallelNode))))
	//}

	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "kube-oidc-proxy e2e suite", r)
}
