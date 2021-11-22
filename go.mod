module github.com/jetstack/kube-oidc-proxy

go 1.13

require (
	github.com/golang/mock v1.6.0
	github.com/heptiolabs/healthcheck v0.0.0-20180807145615-6ff867650f40
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/sebest/xff v0.0.0-20210106013422-671bd2870b3a
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
	gopkg.in/DATA-DOG/go-sqlmock.v1 v1.3.0 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0
	k8s.io/api v0.22.4
	k8s.io/apimachinery v0.22.4
	k8s.io/apiserver v0.22.4
	k8s.io/cli-runtime v0.22.4
	k8s.io/client-go v0.22.4
	k8s.io/component-base v0.22.4
	k8s.io/klog v1.0.0
	sigs.k8s.io/kind v0.11.1
)
