// Copyright Jetstack Ltd. See LICENSE for details.
package probe

import (
	"errors"
	"sync"

	"github.com/heptiolabs/healthcheck"
)

type HealthCheck struct {
	handler healthcheck.Handler
	mu      sync.Mutex
	ready   bool
}

func New() *HealthCheck {
	h := &HealthCheck{
		handler: healthcheck.NewHandler(),
	}

	h.handler.AddReadinessCheck("secure serving", h.Check)

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
