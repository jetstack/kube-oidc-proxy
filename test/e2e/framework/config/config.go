// Copyright Jetstack Ltd. See LICENSE for details.
package config

import (
	"errors"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/jetstack/kube-oidc-proxy/test/environment"
)

type Config struct {
	KubeConfigPath string
	Kubectl        string

	RepoRoot string

	Environment *environment.Environment
}

func (c *Config) Validate() error {
	var errs []error

	if c.KubeConfigPath == "" {
		errs = append(errs, errors.New("kubeconfig path not defined"))
	}

	if c.RepoRoot == "" {
		errs = append(errs, errors.New("repo root not defined"))
	}

	return utilerrors.NewAggregate(errs)
}
