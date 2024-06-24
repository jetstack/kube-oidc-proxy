// Copyright Jetstack Ltd. See LICENSE for details.
package options

import (
	"time"

	"github.com/spf13/pflag"
	cliflag "k8s.io/component-base/cli/flag"

	"github.com/jetstack/kube-oidc-proxy/pkg/util/flags"
)

type KubeOIDCProxyOptions struct {
	DisableImpersonation bool
	ReadinessProbePort   int
	MetricsListenAddress string

	FlushInterval time.Duration

	ExtraHeaderOptions ExtraHeaderOptions
	TokenPassthrough   TokenPassthroughOptions
}

type TokenPassthroughOptions struct {
	Audiences []string
	Enabled   bool
}

type ExtraHeaderOptions struct {
	EnableClientIPExtraUserHeader bool

	ExtraUserHeaders map[string][]string
}

func NewKubeOIDCProxyOptions(nfs *cliflag.NamedFlagSets) *KubeOIDCProxyOptions {
	return new(KubeOIDCProxyOptions).AddFlags(nfs.FlagSet("Kube-OIDC-Proxy"))
}

func (k *KubeOIDCProxyOptions) AddFlags(fs *pflag.FlagSet) *KubeOIDCProxyOptions {
	fs.BoolVar(&k.DisableImpersonation, "disable-impersonation", k.DisableImpersonation,
		"(Alpha) Disable the impersonation of authenticated requests. All "+
			"authenticated requests will be forwarded as is.")

	fs.IntVarP(&k.ReadinessProbePort, "readiness-probe-port", "P", 8080,
		"Port to expose readiness probe.")

	fs.StringVar(&k.MetricsListenAddress, "metrics-serving-address", "0.0.0.0:80",
		"Address to serve metrics on at the /metrics path. An empty address will "+
			"disable serving metrics. Cannot use the same address as proxy or probe.")

	fs.DurationVar(&k.FlushInterval, "flush-interval", time.Millisecond*50,
		"Specifies the interval to flush request bodies. If 0ms, "+
			"no periodic flushing is done. A negative value means to flush "+
			"immediately after each write. Streaming requests such as 'kubectl exec' "+
			"will ignore this option and flush immediately.")

	k.TokenPassthrough.AddFlags(fs)
	k.ExtraHeaderOptions.AddFlags(fs)

	return k
}

func (t *TokenPassthroughOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringSliceVar(&t.Audiences, "token-passthrough-audiences", t.Audiences, ""+
		"(Alpha) List of the identifiers that the resource server presented with the token "+
		"identifies as. The resource server will verify that non OIDC tokens are intended "+
		"for at least one of the audiences in this list. If no audiences are "+
		"provided, the audience will default to the audience of the Kubernetes "+
		"apiserver. Only used when --token-passthrough is also enabled.")

	fs.BoolVar(&t.Enabled, "token-passthrough", t.Enabled, ""+
		"(Alpha) Requests with Bearer tokens that fail OIDC validation are tried against "+
		"the API server using the Token Review endpoint. If successful, the request "+
		"is sent on as is, with no impersonation.")
}

func (e *ExtraHeaderOptions) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&e.EnableClientIPExtraUserHeader, "extra-user-header-client-ip",
		e.EnableClientIPExtraUserHeader, "(Alpha) If enabled, proxied requests will "+
			"include the extra user header 'Impersonate-Extra-Remote-Client-IP: "+
			"<REMOTE_ADDR>' where <REMOTE_ADDR> will contain the remote address of "+
			"the source of the request.")

	fs.Var(flags.NewStringToStringSliceValue(&e.ExtraUserHeaders), "extra-user-headers",
		"(Alpha) A list of key value pairs of extra user headers to pass with "+
			"proxied requests as part of the impersonated request. A single key can "+
			"hold multiple values.")
}
