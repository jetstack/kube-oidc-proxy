module github.com/jetstack/kube-oidc-proxy

go 1.13

require (
	github.com/Masterminds/semver v1.5.0
	github.com/golang/mock v1.3.1
	github.com/heptiolabs/healthcheck v0.0.0-20180807145615-6ff867650f40
	github.com/onsi/ginkgo v1.10.3
	github.com/onsi/gomega v1.7.1
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	gopkg.in/DATA-DOG/go-sqlmock.v1 v1.3.0 // indirect
	gopkg.in/square/go-jose.v2 v2.4.0
	k8s.io/api v0.0.0-20191112020540-7f9008e52f64
	k8s.io/apimachinery v0.0.0-20191111054156-6eb29fdf75dc
	k8s.io/apiserver v0.0.0-20190721103406-1e59c150c171
	k8s.io/cli-runtime v0.0.0-20191111063502-aa6580445795
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/component-base v0.0.0-20191111061729-cca8f4f7ce4d
	k8s.io/klog v1.0.0
	sigs.k8s.io/kind v0.5.1
)

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
