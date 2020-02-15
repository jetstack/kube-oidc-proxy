// Copyright Jetstack Ltd. See LICENSE for details.
package options

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apiserver/pkg/util/term"
	cliflag "k8s.io/component-base/cli/flag"

	"github.com/jetstack/kube-oidc-proxy/pkg/util"
)

const (
	AppName = "kube-oidc-proxy"
)

type Options struct {
	OIDCAuthentication *OIDCAuthenticationOptions
	SecureServing      *SecureServingOptions
	Client             *ClientOptions
	Audit              *AuditOptions
	App                *KubeOIDCProxyOptions
	Misc               *MiscOptions

	nfs *cliflag.NamedFlagSets
}

func New() *Options {
	nfs := new(cliflag.NamedFlagSets)

	// Add flags to command sets
	return &Options{
		OIDCAuthentication: NewOIDCAuthenticationOptions(nfs),
		SecureServing:      NewSecureServingOptions(nfs),
		Client:             NewClientOptions(nfs),
		Audit:              NewAuditOptions(nfs),
		App:                NewKubeOIDCProxyOptions(nfs),
		Misc:               NewMiscOptions(nfs),

		nfs: nfs,
	}
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	// pretty output from kube-apiserver
	usageFmt := "Usage:\n  %s\n"
	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		fmt.Fprintf(cmd.OutOrStderr(), usageFmt, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStderr(), *o.nfs, cols)
		return nil
	})

	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStdout(), *o.nfs, cols)
	})

	fs := cmd.Flags()
	for _, f := range o.nfs.FlagSets {
		fs.AddFlagSet(f)
	}
}

func (o *Options) Validate(cmd *cobra.Command) error {
	if cmd.Flag("version").Value.String() == "true" {
		o.Misc.PrintVersionAndExit()
	}

	var errs []error
	if err := o.OIDCAuthentication.Validate(); err != nil {
		errs = append(errs, err)
	}

	if err := o.SecureServing.Validate(); len(err) > 0 {
		errs = append(errs, err...)
	}

	if err := o.Audit.Validate(); len(err) > 0 {
		errs = append(errs, err...)
	}

	if o.SecureServing.BindPort == o.App.ReadinessProbePort {
		errs = append(errs, errors.New("unable to securely serve on port 8080 (used by readiness probe)"))
	}

	if o.Audit.DynamicConfigurationFlagChanged(cmd) {
		errs = append(errs, errors.New("The flag --audit-dynamic-configuration may not be set"))
	}

	if len(errs) > 0 {
		return util.JoinErrors(errs)
	}

	return nil
}
