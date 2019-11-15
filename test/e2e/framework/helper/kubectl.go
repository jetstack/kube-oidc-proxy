// Copyright Jetstack Ltd. See LICENSE for details.
package helper

import (
	"os"
	"os/exec"
	"strings"
)

type Kubectl struct {
	namespace  string
	kubectl    string
	kubeconfig string
}

func (k *Kubectl) Describe(resources ...string) error {
	resourceNames := strings.Join(resources, ",")
	return k.Run("describe", resourceNames)
}

func (k *Kubectl) DescribeResource(resource, name string) error {
	return k.Run("describe", resource, name)
}

func (h *Helper) Kubectl(ns string) *Kubectl {
	return &Kubectl{
		namespace:  ns,
		kubectl:    h.cfg.Kubectl,
		kubeconfig: h.cfg.KubeConfigPath,
	}
}

func (k *Kubectl) Run(args ...string) error {
	baseArgs := []string{"--kubeconfig", k.kubeconfig}
	if k.namespace == "" {
		baseArgs = append(baseArgs, "--all-namespaces")
	} else {
		baseArgs = []string{"--namespace", k.namespace}
	}
	args = append(baseArgs, args...)
	cmd := exec.Command(k.kubectl, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
