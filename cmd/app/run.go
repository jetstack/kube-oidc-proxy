// Copyright Jetstack Ltd. See LICENSE for details.
package app

import (
	"strconv"

	"github.com/spf13/cobra"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/rest"
	"k8s.io/klog"

	"github.com/jetstack/kube-oidc-proxy/cmd/app/options"
	"github.com/jetstack/kube-oidc-proxy/pkg/metrics"
	"github.com/jetstack/kube-oidc-proxy/pkg/probe"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/hooks"
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

			// Initialise hooks handler
			hooks := hooks.New()
			defer func() {
				if err := hooks.RunPreShutdownHooks(); err != nil {
					klog.Errorf("failed to run shut down hooks: %s", err)
				}
			}()

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

			// Initialise metrics handler
			metrics := metrics.New()
			hooks.AddPreShutdownHook("Metrics", metrics.Shutdown)

			// Initialise token reviewer if enabled
			var tokenReviewer *tokenreview.TokenReview
			if opts.App.TokenPassthrough.Enabled {
				tokenReviewer, err = tokenreview.New(restConfig, metrics, opts.App.TokenPassthrough.Audiences)
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
				tokenReviewer, secureServingInfo, hooks, metrics, proxyConfig)
			if err != nil {
				return err
			}

			// Create a fake JWT to set up readiness probe
			fakeJWT, err := util.FakeJWT(opts.OIDCAuthentication.IssuerURL)
			if err != nil {
				return err
			}

			// Start readiness probe
			readinessHandler, err := probe.Run(
				strconv.Itoa(opts.App.ReadinessProbePort), fakeJWT, p.OIDCTokenAuthenticator())
			if err != nil {
				return err
			}
			hooks.AddPreShutdownHook("Readiness Probe", readinessHandler.Shutdown)

			if len(opts.App.MetricsListenAddress) > 0 {
				if err := metrics.Start(opts.App.MetricsListenAddress); err != nil {
					return err
				}
			} else {
				klog.Info("metrics listen address empty, disabling serving")
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
}
