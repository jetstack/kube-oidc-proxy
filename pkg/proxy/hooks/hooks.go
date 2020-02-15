// Copyright Jetstack Ltd. See LICENSE for details.
package hooks

import (
	"fmt"
	"sync"

	"github.com/jetstack/kube-oidc-proxy/pkg/util"
)

type Hooks struct {
	preShutdownHooks    map[string]ShutdownHook
	preShutdownHookLock sync.Mutex
}

type ShutdownHook func() error

func New() *Hooks {
	return &Hooks{
		preShutdownHooks: make(map[string]ShutdownHook),
	}
}

func (h *Hooks) AddPreShutdownHook(name string, hook ShutdownHook) {
	h.preShutdownHookLock.Lock()
	defer h.preShutdownHookLock.Unlock()

	h.preShutdownHooks[name] = hook
}

// RunPreShutdownHooks runs the PreShutdownHooks for the server
func (h *Hooks) RunPreShutdownHooks() error {
	var errs []error

	h.preShutdownHookLock.Lock()
	defer h.preShutdownHookLock.Unlock()

	for name, entry := range h.preShutdownHooks {
		if err := entry(); err != nil {
			errs = append(errs, fmt.Errorf("PreShutdownHook %q failed: %v", name, err))
		}
	}

	return util.JoinErrors(errs)
}
