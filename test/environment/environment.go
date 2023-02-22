// Copyright Jetstack Ltd. See LICENSE for details.
package environment

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/kind/pkg/cluster/nodes"

	"github.com/jetstack/kube-oidc-proxy/test/kind"
)

const (
	defaultNodeImage = "1.26.0"
	defaultRootPath  = "../../."
)

type Environment struct {
	kind *kind.Kind

	rootPath  string
	nodeImage string
}

func New(masterNodesCount, workerNodesCount int) (*Environment, error) {
	nodeImage := os.Getenv("KUBE_OIDC_PROXY_K8S_VERSION")
	if nodeImage == "" {
		nodeImage = defaultNodeImage
	}
	nodeImage = fmt.Sprintf("kindest/node:v%s", nodeImage)

	rootPath, err := RootPath()
	if err != nil {
		return nil, err
	}

	kind := kind.New(rootPath, nodeImage, masterNodesCount, workerNodesCount)

	return &Environment{
		rootPath:  rootPath,
		nodeImage: nodeImage,
		kind:      kind,
	}, nil
}

func (e *Environment) Create() error {
	if err := e.kind.Create(); err != nil {
		return fmt.Errorf("failed to create kind cluster: %s", err)
	}

	if err := e.kind.LoadAllImages(); err != nil {
		return err
	}

	return nil
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

func (e *Environment) RootPath() string {
	return e.rootPath
}

func (e *Environment) Node(name string) (*nodes.Node, error) {
	ns, err := e.kind.Nodes()
	if err != nil {
		return nil, err
	}

	var node *nodes.Node
	for _, n := range ns {
		if n.String() == name {
			node = &n
			break
		}
	}

	if node == nil {
		return nil, fmt.Errorf("failed to find node %q", name)
	}

	return node, nil
}

func (e *Environment) Kind() *kind.Kind {
	return e.kind
}

func RootPath() (string, error) {
	rootPath := os.Getenv("KUBE_OIDC_PROXY_ROOT_PATH")
	if rootPath == "" {
		rootPath = defaultRootPath
	}

	rootPath, err := filepath.Abs(rootPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path %q: %s",
			rootPath, err)
	}

	return rootPath, nil
}
