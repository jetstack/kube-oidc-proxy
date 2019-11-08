// Copyright Jetstack Ltd. See LICENSE for details.
package environment

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/kind/pkg/cluster/nodes"

	"github.com/jetstack/kube-oidc-proxy/test/kind"
)

const (
	defaultNodeImage = "1.16.1"
)

type Environment struct {
	kind *kind.Kind
}

func Create(masterNodes, workerNodes int) (*Environment, error) {
	nodeImage := os.Getenv("KUBE-OIDC-PROXY_K8S_VERSION")
	if nodeImage == "" {
		nodeImage = defaultNodeImage
	}
	nodeImage = fmt.Sprintf("kindest/node:v%s", nodeImage)

	k, err := kind.New(nodeImage, masterNodes, workerNodes)
	if err != nil {
		return nil, fmt.Errorf("failed to create kind cluster: %s", err)
	}

	return &Environment{
		kind: k,
	}, nil
}

func (e *Environment) Destory() error {
	if e.kind != nil {
		if err := e.kind.Destroy(); err != nil {
			return err
		}
	}

	return nil
}

func (e *Environment) KubeClient() *kubernetes.Clientset {
	return e.kind.KubeClient()
}

func (e *Environment) KubeConfigPath() string {
	return e.kind.KubeConfigPath()
}

func (e *Environment) Node(name string) (*nodes.Node, error) {
	ns, err := e.kind.Nodes()
	if err != nil {
		return nil, err
	}

	var node *nodes.Node
	for _, n := range ns {
		if n.Name() == name {
			node = &n
			break
		}
	}

	if node == nil {
		return nil, fmt.Errorf("failed to find node %q", name)
	}

	return node, nil
}
