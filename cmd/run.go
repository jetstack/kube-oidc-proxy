// Copyright Jetstack Ltd. See LICENSE for details.
package cmd

import (
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
	appName = "kube-oidc-proxy"
)

func NewRunCommand(stopCh <-chan struct{}) *cobra.Command {
	// flag option structs
	oidcOptions := &kubeapiserveroptions.BuiltInAuthenticationOptions{
		OIDC: &kubeapiserveroptions.OIDCAuthenticationOptions{},
	}
	secureServingOptions := apiserveroptions.NewSecureServingOptions()
	secureServingOptions.ServerCert.PairName = appName
	clientConfigFlags := genericclioptions.NewConfigFlags()

	var skipCertReload bool
	var certReloadDuration, readinessProbePort int

	// proxy command
	cmd := &cobra.Command{
		Use:  "k8s-oidc-proxy",
		Long: "k8s-oidc-proxy is a reverse proxy to authenticate users to Kubernetes API servers with Open ID Connect Authentication.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flag("version").Value.String() == "true" {
				version.PrintVersionAndExit()
			}

			if certReloadDuration < 1 {
				return fmt.Errorf("--secret-watch-refresh-period must be a value of 1 or more seconds, got %ds",
					certReloadDuration)
			}

			if secureServingOptions.BindPort == readinessProbePort {
				return fmt.Errorf("cannot both securely serve and expose readiness probe to same port %d",
					readinessProbePort)
			}

			healthCheck, err := probe.New(strconv.Itoa(readinessProbePort))
			if err != nil {
				return fmt.Errorf("failed to initialise readiness probe: %s", err)
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

			// generate self signed certs if required
			if err := secureServingOptions.MaybeDefaultWithSelfSignedCerts(
				"localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
				return err
			}

			//secure serving info has a Serve( function
			secureServingInfo := new(server.SecureServingInfo)
			if err := secureServingOptions.ApplyTo(&secureServingInfo); err != nil {
				return err
			}

			if !skipCertReload {
				// set up secrets watcher
				if err := utils.WatchSecretFiles(restConfig, &oidcOptions.OIDC.CAFile,
					clientConfigFlags.KubeConfig, secureServingOptions,
					time.Second*time.Duration(certReloadDuration)); err != nil {
					return fmt.Errorf("failed to watch secret files: %s", err)
				}
			}

			p := proxy.New(restConfig, reqAuther, secureServingInfo)

			// run proxy
			if err := p.Run(stopCh); err != nil {
				return err
			}

			time.Sleep(time.Second * 3)
			healthCheck.SetReady()

			// stop signal
			<-stopCh

			// stop accepting traffic
			healthCheck.SetNotReady()

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

	// kube-oidc-proxy flags
	namedFlagSets.FlagSet(appName).BoolVarP(
		&skipCertReload, "skip-exit-on-secret-reload", "S", false,
		"If true, kube-oidc-proxy will continue serving when client or serving secrets have changed, instead of exiting gracefully.",
	)
	namedFlagSets.FlagSet(appName).IntVarP(
		&certReloadDuration, "secret-watch-refresh-period", "R", 60,
		"Duration in seconds between checking changes in secret files. Flag is ignored if --skip-exit-on-secret-reload is provided.",
	)
	namedFlagSets.FlagSet(appName).IntVarP(
		&readinessProbePort, "readiness-probe-port", "P", 8080,
		"Port to expose Kubernetes readiness probe.",
	)
	namedFlagSets.FlagSet(appName).SortFlags = true

	// misc flags
	globalflag.AddGlobalFlags(namedFlagSets.FlagSet("misc"), cmd.Name())
	namedFlagSets.FlagSet("misc").Bool("version",
		false, "Print version information and quit")

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
