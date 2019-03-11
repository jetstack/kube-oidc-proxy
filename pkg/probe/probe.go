// Copyright Jetstack Ltd. See LICENSE for details.
package probe

import (
	"errors"
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

func New(port string) *HealthCheck {
	h := &HealthCheck{
		handler: healthcheck.NewHandler(),
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

	return h
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
