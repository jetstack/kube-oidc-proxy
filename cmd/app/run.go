// Copyright Jetstack Ltd. See LICENSE for details.
package app

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/spf13/cobra"
	"k8s.io/apiserver/pkg/server"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/util/term"
	"k8s.io/client-go/rest"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/cli/globalflag"

	"github.com/jetstack/kube-oidc-proxy/cmd/app/options"
	"github.com/jetstack/kube-oidc-proxy/pkg/probe"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/tokenreview"
	"github.com/jetstack/kube-oidc-proxy/pkg/util"
	"github.com/jetstack/kube-oidc-proxy/pkg/version"
)

const (
	appName = "kube-oidc-proxy"
)

func NewRunCommand(stopCh <-chan struct{}) *cobra.Command {
	// flag option structs
	oidcOptions := new(options.OIDCAuthenticationOptions)

	ssoptions := &apiserveroptions.SecureServingOptions{
		BindAddress: net.ParseIP("0.0.0.0"),
		BindPort:    6443,
		Required:    true,
		ServerCert: apiserveroptions.GeneratableKeyCert{
			PairName:      appName,
			CertDirectory: "/var/run/kubernetes",
		},
	}
	ssoptionsWithLB := ssoptions.WithLoopback()

	kopOptions := new(options.KubeOIDCProxyOptions)

	clientConfigOptions := options.NewClientFlags()

	// proxy command
	cmd := &cobra.Command{
		Use:  appName,
		Long: "kube-oidc-proxy is a reverse proxy to authenticate users to Kubernetes API servers with Open ID Connect Authentication.",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error

			if cmd.Flag("version").Value.String() == "true" {
				version.PrintVersionAndExit()
			}

			if err := oidcOptions.Validate(); err != nil {
				return err
			}

			if ssoptionsWithLB.SecureServingOptions.BindPort == kopOptions.ReadinessProbePort {
				return errors.New("unable to securely serve on port 8080, used by readiness prob")
			}

			var restConfig *rest.Config
			if clientConfigOptions.ClientFlagsChanged(cmd) {
				// one or more client flags have been set to use client flag built
				// config
				restConfig, err = clientConfigOptions.ToRESTConfig()
				if err != nil {
					return err
				}

			} else {
				// no client flags have been set so default to in-cluster config
				restConfig, err = rest.InClusterConfig()
				if err != nil {
					return err
				}
			}

			// Initialise token reviewer if enabled
			var tokenReviewer *tokenreview.TokenReview
			if kopOptions.TokenPassthrough.Enabled {
				tokenReviewer, err = tokenreview.New(restConfig, kopOptions.TokenPassthrough.Audiences)
				if err != nil {
					return err
				}
			}

			// Initialise Secure Serving Config
			secureServingInfo := new(server.SecureServingInfo)
			if err := ssoptionsWithLB.ApplyTo(&secureServingInfo, nil); err != nil {
				return err
			}

			proxyOptions := &proxy.Options{
				TokenReview:          kopOptions.TokenPassthrough.Enabled,
				DisableImpersonation: kopOptions.DisableImpersonation,

				ExtraUserHeaders:                kopOptions.ExtraHeaderOptions.ExtraUserHeaders,
				ExtraUserHeadersClientIPEnabled: kopOptions.ExtraHeaderOptions.EnableClientIPExtraUserHeader,
			}

			// Initialise proxy with OIDC token authenticator
			p, err := proxy.New(restConfig, oidcOptions,
				tokenReviewer, secureServingInfo, proxyOptions)
			if err != nil {
				return err
			}

			// Create a fake JWT to set up readiness probe
			fakeJWT, err := util.FakeJWT(oidcOptions.IssuerURL, oidcOptions.APIAudiences)
			if err != nil {
				return err
			}

			// Start readiness probe
			if err := probe.Run(strconv.Itoa(kopOptions.ReadinessProbePort),
				fakeJWT, p.OIDCTokenAuthenticator()); err != nil {
				return err
			}

			// Run proxy
			waitCh, err := p.Run(stopCh)
			if err != nil {
				return err
			}

			<-waitCh

			return nil
		},
	}

	// add flags to command sets
	var namedFlagSets cliflag.NamedFlagSets
	fs := cmd.Flags()

	kopfs := namedFlagSets.FlagSet("Kube-OIDC-Proxy")
	kopOptions.AddFlags(kopfs)

	oidcfs := namedFlagSets.FlagSet("OIDC")
	oidcOptions.AddFlags(oidcfs)

	ssoptionsWithLB.AddFlags(namedFlagSets.FlagSet("Secure Serving"))

	clientConfigOptions.CacheDir = nil
	clientConfigOptions.Impersonate = nil
	clientConfigOptions.ImpersonateGroup = nil
	clientConfigOptions.AddFlags(namedFlagSets.FlagSet("Client"))

	globalflag.AddGlobalFlags(namedFlagSets.FlagSet("Misc"), cmd.Name())
	namedFlagSets.FlagSet("Misc").Bool("version",
		false, "Print version information and quit")

	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}

	// pretty output from kube-apiserver
	usageFmt := "Usage:\n  %s\n"
	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		fmt.Fprintf(cmd.OutOrStderr(), usageFmt, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStderr(), namedFlagSets, cols)
		return nil
	})
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStdout(), namedFlagSets, cols)
	})

	return cmd
}
