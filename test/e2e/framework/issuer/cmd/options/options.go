package options

import (
	"github.com/spf13/cobra"
)

type Options struct {
	BindAddress string
	ListenPort  string

	IssuerURL string

	KeyFile  string
	CertFile string
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&o.BindAddress, "bind-address",
		"0.0.0.0", "Bound Address to listen and serve on.")

	cmd.PersistentFlags().StringVar(&o.ListenPort, "secure-port",
		"6443", "Port to serve HTTPS.")
	cmd.MarkPersistentFlagRequired("secure-port")

	cmd.PersistentFlags().StringVar(&o.IssuerURL, "issuer-url",
		"", "URL of the issuer that appears in well-known responses.")
	cmd.MarkPersistentFlagRequired("issuer-url")

	cmd.PersistentFlags().StringVar(&o.KeyFile, "tls-private-key-file",
		"/etc/oidc/key.pem", "File location to key for serving.")
	cmd.MarkPersistentFlagRequired("tls-private-key-file")

	cmd.PersistentFlags().StringVar(&o.CertFile, "tls-cert-file",
		"/etc/oidc/key.pem", "File location to certificate for serving.")
	cmd.MarkPersistentFlagRequired("tls-cert-file")
}
