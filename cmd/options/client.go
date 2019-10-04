// Copyright Jetstack Ltd. See LICENSE for details.
package options

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	flagInClusterConfig = "in-cluster-config"
)

type ClientExtraOptions struct {
	InClusterConfig bool
	*genericclioptions.ConfigFlags
}

func NewClientExtraFlags() *ClientExtraOptions {
	return &ClientExtraOptions{
		InClusterConfig: false,
		ConfigFlags:     genericclioptions.NewConfigFlags(true),
	}
}

func (c *ClientExtraOptions) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&c.InClusterConfig, flagInClusterConfig, c.InClusterConfig, "Use in-cluster configuration to authenticate and connect to a Kubernetes API server")
	c.ConfigFlags.AddFlags(flags)
}

func (c *ClientExtraOptions) Validate(cmd *cobra.Command) error {
	clientFCh := c.clientFlagsChanged(cmd)

	if clientFCh && c.InClusterConfig {
		return fmt.Errorf("if --%s is enabled, no other client flag options my be specified", flagInClusterConfig)
	}

	if !clientFCh && !c.InClusterConfig {
		return errors.New("no client flag options specified")
	}

	return nil
}

func (c *ClientExtraOptions) clientFlagsChanged(cmd *cobra.Command) bool {
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
