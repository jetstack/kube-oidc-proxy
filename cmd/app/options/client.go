// Copyright Jetstack Ltd. See LICENSE for details.
package options

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type ClientOptions struct {
	*genericclioptions.ConfigFlags
}

func NewClientFlags() *ClientOptions {
	return &ClientOptions{
		ConfigFlags: genericclioptions.NewConfigFlags(true),
	}
}

func (c *ClientOptions) AddFlags(flags *pflag.FlagSet) {
	c.ConfigFlags.AddFlags(flags)
}

func (c *ClientOptions) ClientFlagsChanged(cmd *cobra.Command) bool {
	for _, f := range clientOptionFlags() {
		if ff := cmd.Flag(f); ff != nil && ff.Changed {
			return true
		}
	}

	return false
}

func clientOptionFlags() []string {
	return []string{"certificate-authority", "client-certificate", "client-key", "cluster",
		"context", "insecure-skip-tls-verify", "kubeconfig", "namespace",
		"request-timeout", "server", "token", "user",
	}
}
