module github.com/jetstack/kube-oidc-proxy

go 1.13

require (
	github.com/golang/mock v1.2.0
	github.com/heptiolabs/healthcheck v0.0.0-20180807145615-6ff867650f40
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.7.0
	github.com/sebest/xff v0.0.0-20160910043805-6c115e0ffa35
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	gopkg.in/DATA-DOG/go-sqlmock.v1 v1.3.0 // indirect
	gopkg.in/square/go-jose.v2 v2.3.1
	k8s.io/api v0.18.0
	k8s.io/apimachinery v0.18.0
	k8s.io/apiserver v0.18.0
	k8s.io/cli-runtime v0.18.0
	k8s.io/client-go v0.18.0
	k8s.io/component-base v0.18.0
	k8s.io/klog v1.0.0
	sigs.k8s.io/kind v0.7.0
)
