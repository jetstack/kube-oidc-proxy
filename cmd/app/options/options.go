// Copyright Jetstack Ltd. See LICENSE for details.
package options

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	k8sErrors "k8s.io/apimachinery/pkg/util/errors"
	cliflag "k8s.io/component-base/cli/flag"
)

const (
	AppName = "kube-oidc-proxy"
)

type Options struct {
	App                *KubeOIDCProxyOptions
	OIDCAuthentication *OIDCAuthenticationOptions
	SecureServing      *SecureServingOptions
	Audit              *AuditOptions
	Client             *ClientOptions
	Misc               *MiscOptions

	nfs *cliflag.NamedFlagSets
}

func New() *Options {
	nfs := new(cliflag.NamedFlagSets)

	// Add flags to command sets
	return &Options{
		App:                NewKubeOIDCProxyOptions(nfs),
		OIDCAuthentication: NewOIDCAuthenticationOptions(nfs),
		SecureServing:      NewSecureServingOptions(nfs),
		Audit:              NewAuditOptions(nfs),
		Client:             NewClientOptions(nfs),
		Misc:               NewMiscOptions(nfs),

		nfs: nfs,
	}
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	// pretty output from kube-apiserver
	usageFmt := "Usage:\n  %s\n"

	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		fmt.Fprintf(cmd.OutOrStderr(), usageFmt, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStderr(), *o.nfs, 0)
		return nil
	})

	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStdout(), *o.nfs, 0)
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

	if o.SecureServing.BindPort == o.App.ReadinessProbePort {
		errs = append(errs, errors.New("unable to securely serve on port 8080 (used by readiness probe)"))
	}

	if err := o.Audit.Validate(); len(err) > 0 {
		errs = append(errs, err...)
	}

	if o.App.DisableImpersonation &&
		(o.App.ExtraHeaderOptions.EnableClientIPExtraUserHeader || len(o.App.ExtraHeaderOptions.ExtraUserHeaders) > 0) {
		errs = append(errs, errors.New("cannot add extra user headers when impersonation disabled"))
	}

	if len(errs) > 0 {
		return k8sErrors.NewAggregate(errs)
	}

	return nil
}
