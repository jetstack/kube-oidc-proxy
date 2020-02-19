// Copyright Jetstack Ltd. See LICENSE for details.
package options

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cliflag "k8s.io/component-base/cli/flag"
)

type ClientOptions struct {
	*genericclioptions.ConfigFlags
}

func NewClientOptions(nfs *cliflag.NamedFlagSets) *ClientOptions {
	c := &ClientOptions{
		ConfigFlags: genericclioptions.NewConfigFlags(true),
	}

	// Disable unwanted options
	c.CacheDir = nil
	c.Impersonate = nil
	c.ImpersonateGroup = nil

	return c.AddFlags(nfs.FlagSet("Client"))
}

func (c *ClientOptions) AddFlags(fs *pflag.FlagSet) *ClientOptions {
	c.ConfigFlags.AddFlags(fs)
	return c
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
