// Copyright Jetstack Ltd. See LICENSE for details.
package options

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	cliflag "k8s.io/component-base/cli/flag"
)

type AuditOptions struct {
	*apiserveroptions.AuditOptions
}

func NewAuditOptions(nfs *cliflag.NamedFlagSets) *AuditOptions {
	a := &AuditOptions{
		AuditOptions: apiserveroptions.NewAuditOptions(),
	}

	return a.AddFlags(nfs.FlagSet("Audit"))
}

func (a *AuditOptions) AddFlags(fs *pflag.FlagSet) *AuditOptions {
	a.AuditOptions.AddFlags(fs)
	return a
}

func (a *AuditOptions) DynamicConfigurationFlagChanged(cmd *cobra.Command) bool {
	if ff := cmd.Flag("audit-dynamic-configuration"); ff != nil && ff.Changed {
		return true
	}

	return false
}
