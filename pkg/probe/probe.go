// Copyright Jetstack Ltd. See LICENSE for details.
package probe

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/klog"

	"github.com/jetstack/kube-oidc-proxy/cmd/options"
)

const (
	fakeHeader    = "eyJhbGciOiJSU0EyNTYifQo=" //`{"alg":"RSA256"}`
	fakeSignature = "ZmFrZQo="                 //fake
)

type health struct {
	oidcOptions *options.OIDCAuthenticationOptions
	auther      authenticator.Token

	ready bool
	mu    sync.Mutex
}

func Run(port string, auther authenticator.Token,
	oidcOptions *options.OIDCAuthenticationOptions) error {
	h := &health{
		ready:       false,
		auther:      auther,
		oidcOptions: oidcOptions,
	}

	// setup listener here since it's the best way to check if port is free
	ln, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %s", port, err)
	}

	go func() {
		if err := http.Serve(ln, h); err != nil {
			klog.Errorf("failed to serve ready probe: %s", err)
		}
	}()

	go h.checkReady()

	return nil
}

func (h *health) checkReady() {
	ticker := time.Tick(time.Millisecond * 500)

	for {
		<-ticker

		_, _, err := h.auther.AuthenticateToken(context.Background(), h.buildFakeToken())
		if err != nil {
			klog.Errorf("probe: authenticator not ready: %s", err)
			continue
		}

		klog.Info("kube-oidc-proxy ready")

		h.mu.Lock()
		h.ready = true
		h.mu.Unlock()

		return
	}
}

func (h *health) buildFakeToken() string {
	payloadD := fmt.Sprintf(
		`{iss":"%s", aud":["%s"], %s":"fake", exp":%d}`,
		h.oidcOptions.IssuerURL,
		strings.Join(h.oidcOptions.APIAudiences, `","`),
		h.oidcOptions.UsernameClaim,
		time.Now().Add(time.Second).Unix(),
	)

	payload := base64.StdEncoding.EncodeToString([]byte(payloadD))

	return fmt.Sprintf("%s.%s.%s", fakeHeader, payload, fakeSignature)
}

func (h *health) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	klog.Errorf("probe: authenticator not ready: %s", r.URL.Path)
	if r.URL.Path != "/health" {
		http.Error(rw, "bad path, only /health accepted", http.StatusNotFound)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	status := http.StatusOK
	if !h.ready {
		status = http.StatusServiceUnavailable
	}

	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	rw.WriteHeader(status)

	rw.Write([]byte("{}\n"))
}
