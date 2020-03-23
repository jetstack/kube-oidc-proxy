// Copyright Jetstack Ltd. See LICENSE for details.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jetstack/kube-oidc-proxy/pkg/util"
	"github.com/jetstack/kube-oidc-proxy/test/tools/audit-webhook/cmd/options"
	"github.com/jetstack/kube-oidc-proxy/test/tools/audit-webhook/pkg/sink"
)

func main() {
	opts := new(options.Options)
	stopCh := util.SignalHandler()

	cmd := &cobra.Command{
		Use:   "audit-webhook",
		Short: "An server that implements a Kubernetes audit webhook sink",
		RunE: func(cmd *cobra.Command, args []string) error {

			wh, err := sink.New(opts.LogPath, opts.KeyFile, opts.CertFile, stopCh)
			if err != nil {
				return err
			}

			compCh, err := wh.Run(opts.BindAddress, opts.ListenPort)
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
