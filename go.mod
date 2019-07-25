module github.com/jetstack/kube-oidc-proxy

go 1.12

require (
	github.com/Masterminds/semver v1.4.2
	github.com/golang/mock v0.0.0-20160127222235-bd3c8e81be01
	github.com/heptiolabs/healthcheck v0.0.0-20180807145615-6ff867650f40
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.3
	gopkg.in/square/go-jose.v2 v2.3.1
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/apiserver v0.0.0-20190721103406-1e59c150c171
	k8s.io/cli-runtime v0.0.0
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/component-base v0.0.0
	k8s.io/klog v0.3.3
	sigs.k8s.io/kind v0.4.0
)

replace (
	github.com/golang/mock => github.com/golang/mock v1.3.1
	k8s.io/api => k8s.io/api v0.0.0-20190620084959-7cf5895f2711
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190620085554-14e95df34f1f
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190612205821-1799e75a0719
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20190620085706-2090e6d8f84c
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
	k8s.io/component-base => k8s.io/component-base v0.0.0-20190620085130-185d68e6e6ea
	sigs.k8s.io/kind => sigs.k8s.io/kind v0.4.0
)
