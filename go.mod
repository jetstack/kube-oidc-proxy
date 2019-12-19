module github.com/jetstack/kube-oidc-proxy

go 1.13

require (
	github.com/golang/mock v1.2.0
	github.com/heptiolabs/healthcheck v0.0.0-20180807145615-6ff867650f40
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	golang.org/x/tools v0.0.0-20190621195816-6e04913cbbac // indirect
	gopkg.in/DATA-DOG/go-sqlmock.v1 v1.3.0 // indirect
	gopkg.in/square/go-jose.v2 v2.3.1
	k8s.io/api v0.0.0-20191114100352-16d7abae0d2a
	k8s.io/apimachinery v0.0.0-20191028221656-72ed19daf4bb
	k8s.io/apiserver v0.0.0-20191003001037-3c8b233e046c
	k8s.io/cli-runtime v0.0.0-20191114110141-0a35778df828
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/component-base v0.0.0-20191114102325-35a9586014f7
	k8s.io/klog v1.0.0
	sigs.k8s.io/kind v0.5.1
)

replace (
	github.com/golang/mock => github.com/golang/mock v1.3.1
	k8s.io/api => k8s.io/api v0.0.0-20191114100352-16d7abae0d2a
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20191028221656-72ed19daf4bb
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20191114110141-0a35778df828
	k8s.io/client-go => k8s.io/client-go v0.0.0-20191114101535-6c5935290e33
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20191004115455-8e001e5d1894
	k8s.io/component-base => k8s.io/component-base v0.0.0-20191114102325-35a9586014f7
	k8s.io/metrics => k8s.io/metrics v0.0.0-20191114105837-a4a2842dc51b
	sigs.k8s.io/kind => sigs.k8s.io/kind v0.5.1
)

replace k8s.io/kubectl => k8s.io/kubectl v0.0.0-20190918164019-21692a0861df

replace k8s.io/apiserver => k8s.io/apiserver v0.0.0-20191003001037-3c8b233e046c
