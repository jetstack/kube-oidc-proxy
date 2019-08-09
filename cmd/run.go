// Copyright Jetstack Ltd. See LICENSE for details.
package cmd

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/apiserver/pkg/server"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/util/term"
	"k8s.io/apiserver/plugin/pkg/authenticator/token/oidc"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/cli/globalflag"
	"k8s.io/klog"

	"github.com/jetstack/kube-oidc-proxy/cmd/options"
	"github.com/jetstack/kube-oidc-proxy/pkg/probe"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy"
	"github.com/jetstack/kube-oidc-proxy/pkg/version"
)

const (
	readinessProbePort   = 8080
	informerResyncPeriod = time.Millisecond * 500
	appName              = "kube-oidc-proxy"
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
	auditOptions := apiserveroptions.NewAuditOptions()
	wbhkOptions := apiserveroptions.NewWebhookOptions()
	clientConfigFlags := genericclioptions.NewConfigFlags(true)

	healthCheck := probe.New(strconv.Itoa(readinessProbePort))

	// proxy command
	cmd := &cobra.Command{
		Use:  appName,
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

			if errs := auditOptions.Validate(); len(errs) > 0 {
				return fmt.Errorf("%s", errs)
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
				CAFile:               oidcOptions.CAFile,
				ClientID:             oidcOptions.ClientID,
				GroupsClaim:          oidcOptions.GroupsClaim,
				GroupsPrefix:         oidcOptions.GroupsPrefix,
				IssuerURL:            oidcOptions.IssuerURL,
				RequiredClaims:       oidcOptions.RequiredClaims,
				SupportedSigningAlgs: oidcOptions.SigningAlgs,
				UsernameClaim:        oidcOptions.UsernameClaim,
				UsernamePrefix:       oidcOptions.UsernamePrefix,
			})
			if err != nil {
				return err
			}

			// oidc auther from config
			reqAuther := bearertoken.New(oidcAuther)
			secureServingInfo := new(server.SecureServingInfo)
			if err := ssoptionsWithLB.ApplyTo(&secureServingInfo, nil); err != nil {
				return err
			}

			clientset, err := kubernetes.NewForConfig(restConfig)
			if err != nil {
				return err
			}

			informers := kubeinformers.NewSharedInformerFactory(
				clientset, informerResyncPeriod)
			processInfo := apiserveroptions.NewProcessInfo(
				appName, appName)
			serverConfig := &server.Config{
				ExternalAddress: ssoptionsWithLB.BindAddress.String(),
				SecureServing:   secureServingInfo,
				// Default to treating watch as a long-running operation
				// Generic API servers have no inherent long-running subresources
				LongRunningFunc: genericfilters.BasicLongRunningRequestCheck(
					sets.NewString("watch"), sets.NewString()),
				Authentication: server.AuthenticationInfo{
					APIAudiences:      oidcOptions.APIAudiences,
					Authenticator:     reqAuther,
					SupportsBasicAuth: false,
				},
			}

			auditOptions.ApplyTo(
				serverConfig, restConfig, informers, processInfo, wbhkOptions)

			p := proxy.New(restConfig, reqAuther, serverConfig)

			// run proxy
			waitCh, err := p.Run(stopCh)
			if err != nil {
				return err
			}

			time.Sleep(time.Second * 3)
			healthCheck.SetReady()

			<-waitCh

			if serverConfig.AuditBackend != nil {
				klog.Infof("shutting down audit backend")
				serverConfig.AuditBackend.Shutdown()
			}

			return nil
		},
	}

	// add flags to command sets
	var namedFlagSets cliflag.NamedFlagSets
	fs := cmd.Flags()

	oidcfs := namedFlagSets.FlagSet("OIDC")
	oidcOptions.AddFlags(oidcfs)

	ssoptionsWithLB.AddFlags(namedFlagSets.FlagSet("secure serving"))

	clientConfigFlags.CacheDir = nil
	clientConfigFlags.Impersonate = nil
	clientConfigFlags.ImpersonateGroup = nil
	clientConfigFlags.AddFlags(namedFlagSets.FlagSet("client"))

	auditfs := namedFlagSets.FlagSet("Audit")
	auditOptions.AddFlags(auditfs)

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
