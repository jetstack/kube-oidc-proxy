// Copyright Jetstack Ltd. See LICENSE for details.
package cmd

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/apiserver/pkg/server"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	apiserverflag "k8s.io/apiserver/pkg/util/flag"
	"k8s.io/apiserver/pkg/util/globalflag"
	"k8s.io/apiserver/plugin/pkg/authenticator/token/oidc"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	kubeapiserveroptions "k8s.io/kubernetes/pkg/kubeapiserver/options"

	"github.com/jetstack/kube-oidc-proxy/pkg/probe"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy"
	"github.com/jetstack/kube-oidc-proxy/pkg/utils"
	"github.com/jetstack/kube-oidc-proxy/pkg/version"
)

const (
	readinessProbePort = 8080
)

func NewRunCommand(stopCh <-chan struct{}) *cobra.Command {
	// flag option structs
	oidcOptions := &kubeapiserveroptions.BuiltInAuthenticationOptions{
		OIDC: &kubeapiserveroptions.OIDCAuthenticationOptions{},
	}
	secureServingOptions := apiserveroptions.NewSecureServingOptions()
	secureServingOptions.ServerCert.PairName = "kube-oidc-proxy"
	clientConfigFlags := genericclioptions.NewConfigFlags()

	healthCheck := probe.New(strconv.Itoa(readinessProbePort))

	var certReload bool

	// proxy command
	cmd := &cobra.Command{
		Use:  "k8s-oidc-proxy",
		Long: "k8s-oidc-proxy is a reverse proxy to authenticate users to Kubernetes API servers with Open ID Connect Authentication.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flag("version").Value.String() == "true" {
				version.PrintVersionAndExit()
			}

			if secureServingOptions.BindPort == readinessProbePort {
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

			// oidc config
			oidcAuther, err := oidc.New(oidc.Options{
				APIAudiences:         oidcOptions.APIAudiences,
				CAFile:               oidcOptions.OIDC.CAFile,
				ClientID:             oidcOptions.OIDC.ClientID,
				GroupsClaim:          oidcOptions.OIDC.GroupsClaim,
				GroupsPrefix:         oidcOptions.OIDC.GroupsPrefix,
				IssuerURL:            oidcOptions.OIDC.IssuerURL,
				RequiredClaims:       oidcOptions.OIDC.RequiredClaims,
				SupportedSigningAlgs: oidcOptions.OIDC.SigningAlgs,
				UsernameClaim:        oidcOptions.OIDC.UsernameClaim,
				UsernamePrefix:       oidcOptions.OIDC.UsernamePrefix,
			})
			if err != nil {
				return err
			}

			// oidc auther from config
			reqAuther := bearertoken.New(oidcAuther)

			if secureServingOptions.Validate(); err != nil {
				return err
			}

			if err := secureServingOptions.MaybeDefaultWithSelfSignedCerts(
				"localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
				return err
			}

			//secure serving info has a Serve( function
			secureServingInfo := new(server.SecureServingInfo)
			if err := secureServingOptions.ApplyTo(&secureServingInfo); err != nil {
				return err
			}

			if certReload {
				utils.WatchSecretFiles(restConfig, &oidcOptions.OIDC.CAFile,
					clientConfigFlags.KubeConfig, secureServingOptions, time.Minute)
			}

			p := proxy.New(restConfig, reqAuther, secureServingInfo)

			// run proxy
			if err := p.Run(stopCh); err != nil {
				return err
			}

			time.Sleep(time.Second * 3)
			healthCheck.SetReady()

			<-stopCh

			return nil
		},
	}

	// add flags to command sets
	var namedFlagSets apiserverflag.NamedFlagSets
	fs := cmd.Flags()

	oidcfs := namedFlagSets.FlagSet("OIDC")
	oidcOptions.AddFlags(oidcfs)

	secureServingOptions.AddFlags(namedFlagSets.FlagSet("secure serving"))

	clientConfigFlags.CacheDir = nil
	clientConfigFlags.Impersonate = nil
	clientConfigFlags.ImpersonateGroup = nil
	clientConfigFlags.AddFlags(namedFlagSets.FlagSet("client"))

	globalflag.AddGlobalFlags(namedFlagSets.FlagSet("misc"), cmd.Name())
	namedFlagSets.FlagSet("misc").Bool("version",
		false, "Print version information and quit")

	namedFlagSets.FlagSet("kube-oidc-proxy").BoolVarP(
		&certReload, "exit-on-certificate-reload", "E", true,
		"kube-oidc-proxy will exit gracefully when client or serving secrets have changed",
	)

	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}

	// pretty output from kube-apiserver
	usageFmt := "Usage:\n  %s\n"
	cols, _, _ := apiserverflag.TerminalSize(cmd.OutOrStdout())
	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		fmt.Fprintf(cmd.OutOrStderr(), usageFmt, cmd.UseLine())
		apiserverflag.PrintSections(cmd.OutOrStderr(), namedFlagSets, cols)
		return nil
	})
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		apiserverflag.PrintSections(cmd.OutOrStdout(), namedFlagSets, cols)
	})

	return cmd
}
