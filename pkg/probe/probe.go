// Copyright Jetstack Ltd. See LICENSE for details.
package probe

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/heptiolabs/healthcheck"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/klog"
)

const (
	timeout = time.Second * 10
)

type HealthCheck struct {
	handler healthcheck.Handler

	oidcAuther authenticator.Token
	fakeJWT    string

	ready bool
}

func Run(port, fakeJWT string, oidcAuther authenticator.Token) error {
	h := &HealthCheck{
		handler:    healthcheck.NewHandler(),
		oidcAuther: oidcAuther,
		fakeJWT:    fakeJWT,
	}

	h.handler.AddReadinessCheck("secure serving", h.Check)

	go func() {
		for {
			err := http.ListenAndServe(net.JoinHostPort("0.0.0.0", port), h.handler)
			if err != nil {
				klog.Errorf("ready probe listener failed: %s", err)
			}
			time.Sleep(5 * time.Second)
		}
	}()

	return nil
}

func (h *HealthCheck) Check() error {
	if h.ready {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, _, err := h.oidcAuther.AuthenticateToken(ctx, h.fakeJWT)
	if err != nil && strings.HasSuffix(err.Error(), "authenticator not initialized") {
		err = fmt.Errorf("OIDC provider not yet initialized: %s", err)
		klog.V(4).Infof(err.Error())
		return err
	}

	h.ready = true

	klog.Info("OIDC provider initialized, proxy ready")
	klog.V(4).Infof("OIDC provider initialized, readiness check returned error: %s", err)

	return nil
}
