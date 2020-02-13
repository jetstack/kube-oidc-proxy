// Copyright Jetstack Ltd. See LICENSE for details.
package options

import (
	"github.com/spf13/cobra"
)

type Options struct {
	BindAddress string
	ListenPort  string

	KeyFile  string
	CertFile string
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&o.BindAddress, "bind-address",
		"0.0.0.0", "Bound Address to listen and serve on.")

	cmd.PersistentFlags().StringVar(&o.ListenPort, "secure-port",
		"6443", "Port to serve HTTPS.")
	o.must(cmd.MarkPersistentFlagRequired("secure-port"))

	cmd.PersistentFlags().StringVar(&o.KeyFile, "tls-private-key-file",
		"/etc/apiserver/key.pem", "File location to key for serving.")
	o.must(cmd.MarkPersistentFlagRequired("tls-private-key-file"))

	cmd.PersistentFlags().StringVar(&o.CertFile, "tls-cert-file",
		"/etc/apiserver/key.pem", "File location to certificate for serving.")
	o.must(cmd.MarkPersistentFlagRequired("tls-cert-file"))
}

func (o *Options) must(err error) {
	if err != nil {
		panic(err)
	}
}
