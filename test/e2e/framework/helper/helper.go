// Copyright Jetstack Ltd. See LICENSE for details.
package helper

import (
	"k8s.io/client-go/kubernetes"

	"github.com/jetstack/kube-oidc-proxy/test/e2e/framework/config"
)

// Helper provides methods for common operations needed during tests.
type Helper struct {
	cfg *config.Config

	KubeClient kubernetes.Interface
}

func NewHelper(cfg *config.Config) *Helper {
	return &Helper{
		cfg: cfg,
	}
}
