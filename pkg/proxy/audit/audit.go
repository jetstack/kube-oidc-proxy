// Copyright Jetstack Ltd. See LICENSE for details.
package audit

import (
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/util/sets"
	genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	"k8s.io/apiserver/pkg/server"
	genericfilters "k8s.io/apiserver/pkg/server/filters"

	"github.com/jetstack/kube-oidc-proxy/cmd/app/options"
)

type Audit struct {
	options      *options.AuditOptions
	serverConfig *server.CompletedConfig
}

func New(options *options.AuditOptions, externalAddress string, secureServingInfo *server.SecureServingInfo) (*Audit, error) {
	serverConfig := &server.Config{
		ExternalAddress: externalAddress,
		SecureServing:   secureServingInfo,

		// Default to treating watch as a long-running operation
		// Generic API servers have no inherent long-running subresources
		LongRunningFunc: genericfilters.BasicLongRunningRequestCheck(
			sets.NewString("watch"), sets.NewString()),
	}

	// We do not support dynamic auditing
	if err := options.ApplyTo(serverConfig, nil, nil, nil, nil); err != nil {
		return nil, err
	}

	completed := serverConfig.Complete(nil)

	return &Audit{
		options:      options,
		serverConfig: &completed,
	}, nil
}

func (a *Audit) Run(stopCh <-chan struct{}) error {
	if a.serverConfig.AuditBackend != nil {
		if err := a.serverConfig.AuditBackend.Run(stopCh); err != nil {
			return fmt.Errorf("failed to run the audit backend: %s", err)
		}
	}

	return nil
}

func (a *Audit) Shutdown() error {
	if a.serverConfig.AuditBackend != nil {
		a.serverConfig.AuditBackend.Shutdown()
	}

	return nil
}

func (a *Audit) WithRequest(handler http.Handler) http.Handler {
	handler = genericapifilters.WithAudit(handler, a.serverConfig.AuditBackend, a.serverConfig.AuditPolicyChecker, a.serverConfig.LongRunningFunc)
	return genericapifilters.WithRequestInfo(handler, a.serverConfig.RequestInfoResolver)
}

func (a *Audit) WithUnauthorized(handler http.Handler) http.Handler {
	handler = genericapifilters.WithFailedAuthenticationAudit(handler, a.serverConfig.AuditBackend, a.serverConfig.AuditPolicyChecker)
	return genericapifilters.WithRequestInfo(handler, a.serverConfig.RequestInfoResolver)
}
