// Copyright Jetstack Ltd. See LICENSE for details.
package probe

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"k8s.io/klog"

	"github.com/heptiolabs/healthcheck"
)

type HealthCheck struct {
	handler healthcheck.Handler
	mu      sync.Mutex
	ready   bool
}

func New(port string) (*HealthCheck, error) {
	h := &HealthCheck{
		handler: healthcheck.NewHandler(),
	}

	h.handler.AddReadinessCheck("secure serving", h.Check)

	// Open a listener here to tet that the port is free and we can cleanly exit
	ln, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", port))
	if err != nil {
		return nil, fmt.Errorf("readiness probe failed to listen on port %s: %s",
			port, err)
	}

	if err := ln.Close(); err != nil {
		return nil, fmt.Errorf(
			"failed to close readiness probe port testing listener: %s",
			err)
	}

	// Service the health check
	go func() {
		for {
			err := http.ListenAndServe(net.JoinHostPort("0.0.0.0", port), h.handler)
			if err != nil {
				klog.Errorf("readiness probe listener failed: %s", err)
			}
			time.Sleep(5 * time.Second)
		}
	}()

	return h, nil
}

func (h *HealthCheck) Check() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.ready {
		return errors.New("not ready")
	}

	return nil
}

func (h *HealthCheck) SetReady() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.ready = true
}

func (h *HealthCheck) SetNotReady() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.ready = false
}
