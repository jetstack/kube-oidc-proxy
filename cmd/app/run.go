// Copyright Jetstack Ltd. See LICENSE for details.
package app

import (
	"strconv"

	"github.com/spf13/cobra"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/rest"

	"github.com/jetstack/kube-oidc-proxy/cmd/app/options"
	"github.com/jetstack/kube-oidc-proxy/pkg/probe"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/tokenreview"
	"github.com/jetstack/kube-oidc-proxy/pkg/util"
)

func NewRunCommand(stopCh <-chan struct{}) *cobra.Command {
	// Build options
	opts := options.New()

	// Build command
	cmd := buildRunCommand(stopCh, opts)

	// Add option flags to command
	opts.AddFlags(cmd)

	return cmd
}

// Proxy command
func buildRunCommand(stopCh <-chan struct{}, opts *options.Options) *cobra.Command {
	return &cobra.Command{
		Use:  options.AppName,
		Long: "kube-oidc-proxy is a reverse proxy to authenticate users to Kubernetes API servers with Open ID Connect Authentication.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Validate(cmd); err != nil {
				return err
			}

			// Here we determine to either use custom or 'in-cluster' client configuration
			var err error
			var restConfig *rest.Config
			if opts.Client.ClientFlagsChanged(cmd) {
				// One or more client flags have been set to use client flag built
				// config
				restConfig, err = opts.Client.ToRESTConfig()
				if err != nil {
					return err
				}

			} else {
				// No client flags have been set so default to in-cluster config
				restConfig, err = rest.InClusterConfig()
				if err != nil {
					return err
				}
			}

			// Initialise token reviewer if enabled
			var tokenReviewer *tokenreview.TokenReview
			if opts.App.TokenPassthrough.Enabled {
				tokenReviewer, err = tokenreview.New(restConfig, opts.App.TokenPassthrough.Audiences)
				if err != nil {
					return err
				}
			}

			// Initialise Secure Serving Config
			secureServingInfo := new(server.SecureServingInfo)
			if err := opts.SecureServing.ApplyTo(&secureServingInfo); err != nil {
				return err
			}

			proxyConfig := &proxy.Config{
				TokenReview:          opts.App.TokenPassthrough.Enabled,
				DisableImpersonation: opts.App.DisableImpersonation,

				FlushInterval:   opts.App.FlushInterval,
				ExternalAddress: opts.SecureServing.BindAddress.String(),

				ExtraUserHeaders:                opts.App.ExtraHeaderOptions.ExtraUserHeaders,
				ExtraUserHeadersClientIPEnabled: opts.App.ExtraHeaderOptions.EnableClientIPExtraUserHeader,
			}

			// Initialise proxy with OIDC token authenticator
			p, err := proxy.New(restConfig, opts.OIDCAuthentication, opts.Audit,
				tokenReviewer, secureServingInfo, proxyConfig)
			if err != nil {
				return err
			}

			// Create a fake JWT to set up readiness probe
			fakeJWT, err := util.FakeJWT(opts.OIDCAuthentication.IssuerURL, opts.OIDCAuthentication.APIAudiences)
			if err != nil {
				return err
			}

			// Start readiness probe
			if err := probe.Run(strconv.Itoa(opts.App.ReadinessProbePort),
				fakeJWT, p.OIDCTokenAuthenticator()); err != nil {
				return err
			}

			// Run proxy
			waitCh, err := p.Run(stopCh)
			if err != nil {
				return err
			}

			<-waitCh

			if err := p.RunPreShutdownHooks(); err != nil {
				return err
			}

			return nil
		},
	}
}
