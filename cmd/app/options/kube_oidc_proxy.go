// Copyright Jetstack Ltd. See LICENSE for details.
package options

import (
	"github.com/spf13/pflag"
)

type TokenPassthroughOptions struct {
	Audiences []string
	Enabled   bool
}

type KubeOIDCProxyOptions struct {
	DisableImpersonation bool
	TokenPassthrough     TokenPassthroughOptions

	ReadinessProbePort int
	WatchFiles         []string
	WatchRefreshPeriod string
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

func (k *KubeOIDCProxyOptions) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&k.DisableImpersonation, "disable-impersonation", k.DisableImpersonation,
		"(Alpha) Disable the impersonation of authenticated requests. All "+
			"authenticated requests will be forwarded as is.")

	fs.IntVarP(&k.ReadinessProbePort, "readiness-probe-port", "P", 8080,
		"Port to expose readiness probe.")

	fs.StringSliceVar(&k.WatchFiles, "reload-watch-files", nil,
		"Comma-separated list of files to watch for changes. If a change is "+
			"detected then send self a SIGHUP signal and exit gracefully.")
	fs.StringVar(&k.WatchRefreshPeriod, "reload-watch-refresh-period", "60s",
		"Duration period to test for changes in watched files")

	k.TokenPassthrough.AddFlags(fs)
}
