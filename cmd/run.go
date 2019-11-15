// Copyright Jetstack Ltd. See LICENSE for details.
package cmd

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/spf13/cobra"
	"k8s.io/apiserver/pkg/server"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/util/term"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/cli/globalflag"

	"github.com/jetstack/kube-oidc-proxy/cmd/options"
	"github.com/jetstack/kube-oidc-proxy/pkg/probe"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/tokenreview"
	"github.com/jetstack/kube-oidc-proxy/pkg/version"
)

const (
	readinessProbePort = 8080
)

func NewRunCommand(stopCh <-chan struct{}) *cobra.Command {
	// flag option structs
	oidcOptions := new(options.OIDCAuthenticationOptions)

	ssoptions := &apiserveroptions.SecureServingOptions{
		BindAddress: net.ParseIP("0.0.0.0"),
		BindPort:    6443,
		Required:    true,
		ServerCert: apiserveroptions.GeneratableKeyCert{
			PairName:      "kube-oidc-proxy",
			CertDirectory: "/var/run/kubernetes",
		},
	}
	ssoptionsWithLB := ssoptions.WithLoopback()

	kopOptions := new(options.KubeOIDCProxyOptions)

	clientConfigFlags := genericclioptions.NewConfigFlags(true)

	healthCheck := probe.New(strconv.Itoa(readinessProbePort))

	// proxy command
	cmd := &cobra.Command{
		Use:  "kube-oidc-proxy",
		Long: "kube-oidc-proxy is a reverse proxy to authenticate users to Kubernetes API servers with Open ID Connect Authentication.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flag("version").Value.String() == "true" {
				version.PrintVersionAndExit()
			}

			if err := oidcOptions.Validate(); err != nil {
				return err
			}

			if ssoptionsWithLB.SecureServingOptions.BindPort == readinessProbePort {
				return errors.New("unable to securely serve on port 8080, used by readiness prob")
			}

			// client rest config
			restConfig, err := rest.InClusterConfig()
			if err != nil {

				// fall back to cli flags if in cluster fails
				restConfig, err = clientConfigFlags.ToRESTConfig()
				if err != nil {
					return err
				}
			}

			// Init token reviewer if enabled
			var tokenReviewer *tokenreview.TokenReview
			if kopOptions.TokenPassthrough.Enabled {
				tokenReviewer, err = tokenreview.New(restConfig, kopOptions.TokenPassthrough.Audiences)
				if err != nil {
					return err
				}
			}

			// oidc auther from config
			secureServingInfo := new(server.SecureServingInfo)
			if err := ssoptionsWithLB.ApplyTo(&secureServingInfo, nil); err != nil {
				return err
			}

			proxyOptions := &proxy.Options{
				TokenReview:          kopOptions.TokenPassthrough.Enabled,
				DisableImpersonation: kopOptions.DisableImpersonation,
			}

			p := proxy.New(restConfig, oidcOptions,
				tokenReviewer, secureServingInfo, healthCheck, proxyOptions)

			// run proxy
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

	ssoptionsWithLB.AddFlags(namedFlagSets.FlagSet("secure serving"))

	clientConfigFlags.CacheDir = nil
	clientConfigFlags.Impersonate = nil
	clientConfigFlags.ImpersonateGroup = nil
	clientConfigFlags.AddFlags(namedFlagSets.FlagSet("client"))

	globalflag.AddGlobalFlags(namedFlagSets.FlagSet("misc"), cmd.Name())
	namedFlagSets.FlagSet("misc").Bool("version",
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
