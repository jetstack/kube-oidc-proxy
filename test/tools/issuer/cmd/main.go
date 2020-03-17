// Copyright Jetstack Ltd. See LICENSE for details.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jetstack/kube-oidc-proxy/pkg/util"
	"github.com/jetstack/kube-oidc-proxy/test/tools/issuer/cmd/options"
	"github.com/jetstack/kube-oidc-proxy/test/tools/issuer/pkg/issuer"
)

func main() {
	opts := new(options.Options)
	stopCh := util.SignalHandler()

	cmd := &cobra.Command{
		Use:   "oidc-issuer",
		Short: "A very basic OIDC issuer to present a well-known endpoint.",
		RunE: func(cmd *cobra.Command, args []string) error {

			iss, err := issuer.New(opts.IssuerURL, opts.KeyFile, opts.CertFile, stopCh)
			if err != nil {
				return err
			}

			compCh, err := iss.Run(opts.BindAddress, opts.ListenPort)
			if err != nil {
				return err
			}

			<-compCh

			return nil
		},
	}

	opts.AddFlags(cmd)

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}
