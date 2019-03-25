// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"io/ioutil"
	"os"
	"testing"

	"k8s.io/client-go/tools/clientcmd"

	"k8s.io/klog"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/config"
)

const (
	defaultNodeImage = "1.14.0"
)

var e2eSuite *E2E

func TestMain(m *testing.M) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "kube-oidc-proxy")
	if err != nil {
		klog.Fatalf("failed to create tmp directory: %s", err)
	}

	nodeImage := os.Getenv("KUBE_OIDC_PROXY_NODE_IMAGE")
	if nodeImage == "" {
		nodeImage = defaultNodeImage
	}
	nodeImage = fmt.Sprintf("kindest/node:v%s", nodeImage)

	clusterContext := cluster.NewContext("kube-oidc-proxy-e2e")

	// build default config
	conf := new(config.Cluster)
	config.SetDefaults_Cluster(conf)
	if len(conf.Nodes) == 0 {
		klog.Fatal("kind default config set node count to 0")
	}

	for i := range conf.Nodes {
		conf.Nodes[i].Image = nodeImage
	}

	// create kind cluster
	klog.Infof("creating kind cluster '%s'", clusterContext.Name())
	if err := clusterContext.Create(conf); err != nil {
		klog.Fatalf("error creating cluster: %s", err)
	}

	// generate rest config to kind cluster
	kubeconfig := clusterContext.KubeConfigPath()
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Errorf("failed to build kind rest client: %s", err)
		cleanup(tmpDir, clusterContext, 1)
	}

	e2eSuite = New(kubeconfig, tmpDir, restConfig)
	if err := e2eSuite.Run(); err != nil {
		klog.Errorf("failed to start e2e suite: %s", err)
		cleanup(tmpDir, clusterContext, 1)
	}

	// run tests
	runErr := m.Run()

	// clean up and exit
	e2eSuite.cleanup()
	cleanup(tmpDir, clusterContext, runErr)
}

func cleanup(tmpDir string, clusterContext *cluster.Context, exitCode int) {
	err := os.RemoveAll(tmpDir)
	if err != nil {
		klog.Errorf("failed to delete temp dir %s: %s", tmpDir, err)
	}

	klog.Infof("destroying kind cluster '%s'", clusterContext.Name())
	kindErr := clusterContext.Delete()
	if kindErr != nil {
		klog.Errorf("error destroying kind cluster: %s", err)
	}

	if err != nil || kindErr != nil {
		os.Exit(1)
	}

	os.Exit(exitCode)
}
