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
	*http.Server
	oidcAuther authenticator.Token
	fakeJWT    string

	ready bool
}

func Run(port, fakeJWT string, oidcAuther authenticator.Token) (*HealthCheck, error) {
	handler := healthcheck.NewHandler()

	ln, err := net.Listen("tcp", "0.0.0.0:"+port)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on health check port: %s", err)
	}

	h := &HealthCheck{
		oidcAuther: oidcAuther,
		fakeJWT:    fakeJWT,
		Server: &http.Server{
			Addr:           ln.Addr().String(),
			ReadTimeout:    8 * time.Second,
			WriteTimeout:   8 * time.Second,
			MaxHeaderBytes: 1 << 20, // 1 MiB
			Handler:        handler,
		},
	}

	handler.AddReadinessCheck("secure serving", h.Check)

	go func() {
		klog.Infof("serving readiness probe on %s/ready", ln.Addr())

		if err := h.Serve(ln); err != nil {
			klog.Errorf("failed to serve readiness probe: %s", err)
			return
		}
	}()

	return h, nil
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

	klog.V(4).Infof("OIDC provider initialized, readiness check returned expected error: %s", err)
	klog.Info("OIDC provider initialized, proxy ready")

	return nil
}

func (h *HealthCheck) Shutdown() error {
	// If readiness probe server is not started than exit early
	if h.Server == nil {
		return nil
	}

	klog.Info("shutting down readiness probe server...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := h.Server.Shutdown(ctx); err != nil {
		return fmt.Errorf("readiness probe server shutdown failed: %s", err)
	}

	klog.Info("readines probe server gracefully stopped")

	return nil
}
