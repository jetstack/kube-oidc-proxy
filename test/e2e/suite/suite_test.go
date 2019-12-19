// Copyright Jetstack Ltd. See LICENSE for details.
package suite

import (
	"testing"
	"time"

	"github.com/onsi/ginkgo"
	ginkgoconfig "github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
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

	junitReporter := reporters.NewJUnitReporter("../../../artifacts/junit-go-e2e.xml")
	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "kube-oidc-proxy e2e suite", []ginkgo.Reporter{junitReporter})
}
