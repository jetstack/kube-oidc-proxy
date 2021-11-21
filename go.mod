module github.com/jetstack/kube-oidc-proxy

go 1.13

require (
	github.com/golang/mock v1.4.1
	github.com/heptiolabs/healthcheck v0.0.0-20180807145615-6ff867650f40
	github.com/onsi/ginkgo v1.14.0
	github.com/onsi/gomega v1.10.1
	github.com/sebest/xff v0.0.0-20160910043805-6c115e0ffa35
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	golang.org/x/term v0.0.0-20210220032956-6a3ed077a48d
	gopkg.in/DATA-DOG/go-sqlmock.v1 v1.3.0 // indirect
	gopkg.in/square/go-jose.v2 v2.3.1
	k8s.io/api v0.22.4
	k8s.io/apimachinery v0.22.4
	k8s.io/apiserver v0.22.4
	k8s.io/cli-runtime v0.22.4
	k8s.io/client-go v0.22.4
	k8s.io/component-base v0.22.4
	k8s.io/klog v1.0.0
	sigs.k8s.io/kind v0.11.1
)
