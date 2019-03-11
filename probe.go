package main

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

func NewHealthCheck() *HealthCheck {
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

func (h *HealthCheck) Ready() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.ready = true
}

func (h *HealthCheck) NotReady() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.ready = false
}
