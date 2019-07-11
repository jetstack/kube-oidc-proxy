// Copyright Jetstack Ltd. See LICENSE for details.
package cmd

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/apiserver/pkg/server"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/util/term"
	"k8s.io/apiserver/plugin/pkg/authenticator/token/oidc"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	//clientgoinformers "k8s.io/client-go/informers"
	//"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/keyutil"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/cli/globalflag"
	//serviceaccountcontroller "k8s.io/kubernetes/pkg/controller/serviceaccount"

	"github.com/jetstack/kube-oidc-proxy/cmd/options"
	"github.com/jetstack/kube-oidc-proxy/pkg/probe"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy"
	serviceaccount "github.com/jetstack/kube-oidc-proxy/pkg/proxy/serviceaccount/authenticator"
	"github.com/jetstack/kube-oidc-proxy/pkg/version"
)

const (
	readinessProbePort = 8080

	authPassthroughCLISetName     = "Service Account Passthrough (Alpha)"
	saTokenPassthroughEnabledName = "service-account-token-passthrough"
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

	saOptions := new(options.ServiceAccountAuthenticationOptions)

	var saPassthroughEnabled bool

	secureServingOptions := apiserveroptions.NewSecureServingOptions()
	secureServingOptions.ServerCert.PairName = "kube-oidc-proxy"
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

			if !saPassthroughEnabled {
				//oidcOptions.ServiceAccounts = saOptions.ServiceAccounts
				saOptions = nil
			}

			oidcAuther, saAuther, err := buildAuthenticators(restConfig, oidcOptions, saOptions)
			if err != nil {
				return err
			}

			secureServingInfo := new(server.SecureServingInfo)
			if err := ssoptionsWithLB.ApplyTo(&secureServingInfo, nil); err != nil {
				return err
			}

			p := proxy.New(restConfig, secureServingInfo, oidcAuther, saAuther)

			// run proxy
			waitCh, err := p.Run(stopCh)
			if err != nil {
				return err
			}

			time.Sleep(time.Second * 3)
			healthCheck.SetReady()

			<-waitCh

			return nil
		},
	}

	// add flags to command sets
	var namedFlagSets cliflag.NamedFlagSets
	fs := cmd.Flags()

	oidcfs := namedFlagSets.FlagSet("OIDC")
	oidcOptions.AddFlags(oidcfs)

	safs := namedFlagSets.FlagSet(authPassthroughCLISetName)
	saOptions.AddFlags(safs)

	namedFlagSets.FlagSet(authPassthroughCLISetName).BoolVar(&saPassthroughEnabled,
		saTokenPassthroughEnabledName,
		false,
		"Requests with a Service Account token are forwarded onto the API server"+
			" as is, rather than being rejected. No impersonation takes place."+
			" (Experiential)",
	)

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

// Build OIDC authenticator and SA authenticator if enabled
func buildAuthenticators(restConfig *rest.Config, oidcOptions *options.OIDCAuthenticationOptions,
	saOptions *options.ServiceAccountAuthenticationOptions) (*bearertoken.Authenticator, authenticator.Token, error) {
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
		return nil, nil, err
	}

	// oidc auther from config
	reqAuther := bearertoken.New(oidcAuther)

	var saAuther authenticator.Token
	if saOptions != nil {
		allPublicKeys := []interface{}{}
		for _, keyfile := range saOptions.KeyFiles {
			publicKeys, err := keyutil.PublicKeysFromFile(keyfile)
			if err != nil {
				return nil, nil, err
			}

			allPublicKeys = append(allPublicKeys, publicKeys...)
		}

		var getter serviceaccount.ServiceAccountTokenGetter
		// TODO: Add both legacy and new SA validators.
		// TODO: set up fake getter for new validator if lookup is false.
		if saOptions.Lookup {
			//client, err := kubernetes.NewForConfig(restConfig)
			//if err != nil {
			//	return nil, nil, err
			//}

			//informer := clientgoinformers.NewSharedInformerFactory(client, 10*time.Minute)

			//getter = serviceaccountcontroller.NewGetterFromClient(
			//	client,
			//	informer.Core().V1().Secrets().Lister(),
			//	informer.Core().V1().ServiceAccounts().Lister(),
			//	informer.Core().V1().Pods().Lister(),
			//)
		}

		validator := serviceaccount.NewValidator(getter)
		saAuther = serviceaccount.JWTTokenAuthenticator(saOptions.Issuer, allPublicKeys, oidcOptions.APIAudiences, validator)
	}

	return reqAuther, saAuther, nil
}
