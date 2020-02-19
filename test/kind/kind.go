// Copyright Jetstack Ltd. See LICENSE for details.
package kind

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	configv1alpha4 "sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
)

const (
	clusterName = "kube-oidc-proxy-e2e"
)

type Kind struct {
	rootPath string

	provider   *cluster.Provider
	restConfig *rest.Config
	client     *kubernetes.Clientset
}

func New(rootPath, nodeImage string, masterNodes, workerNodes int) (*Kind, error) {
	log.Infof("kind: using k8s node image %q", nodeImage)

	k := &Kind{
		rootPath: rootPath,
	}

	conf := new(configv1alpha4.Cluster)
	configv1alpha4.SetDefaultsCluster(conf)
	conf.Nodes = nil

	// This behviour will be changing soon in later versions of kind.
	if workerNodes == 0 {
		for i := 0; i < masterNodes; i++ {
			conf.Nodes = append(conf.Nodes,
				configv1alpha4.Node{
					Image: nodeImage,
				})
		}

	} else {
		for i := 0; i < masterNodes; i++ {
			conf.Nodes = append(conf.Nodes,
				configv1alpha4.Node{
					Image: nodeImage,
					Role:  configv1alpha4.ControlPlaneRole,
				})
		}

		for i := 0; i < workerNodes; i++ {
			conf.Nodes = append(conf.Nodes,
				configv1alpha4.Node{
					Image: nodeImage,
					Role:  configv1alpha4.WorkerRole,
				})
		}
	}

	conf.Networking.ServiceSubnet = "10.0.0.0/16"

	// create kind cluster
	log.Infof("kind: creating kind cluster %q", clusterName)
	k.provider = cluster.NewProvider()
	if err := k.provider.Create(
		clusterName,
		cluster.CreateWithV1Alpha4Config(conf),
	); err != nil {
		return nil, err
	}

	// generate rest config to kind cluster
	kubeconfigData, err := k.provider.KubeConfig(clusterName, false)
	if err != nil {
		return nil, err
	}

	if err := ioutil.WriteFile(k.KubeConfigPath(), []byte(kubeconfigData), 0600); err != nil {
		return nil, err
	}

	restConfig, err := clientcmd.BuildConfigFromFlags("", k.KubeConfigPath())
	if err != nil {
		return nil, k.errDestroy(fmt.Errorf("failed to build kind rest client: %s", err))
	}
	k.restConfig = restConfig

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, k.errDestroy(fmt.Errorf("failed to build kind kubernetes client: %s", err))
	}
	k.client = client

	if err := k.waitForNodesReady(); err != nil {
		return nil, k.errDestroy(fmt.Errorf("failed to wait for nodes to become ready: %s", err))
	}

	if err := k.waitForCoreDNSReady(); err != nil {
		return nil, k.errDestroy(fmt.Errorf("failed to wait for DNS pods to become ready: %s", err))
	}

	log.Infof("kind: cluster ready %q", clusterName)

	return k, nil
}

func DeleteCluster(name string) error {
	provider := cluster.NewProvider()

	kubeconfig, err := provider.KubeConfig(clusterName, false)
	if err != nil {
		return err
	}

	return provider.Delete(clusterName, kubeconfig)
}

func (k *Kind) Destroy() error {
	if err := k.collectLogs(); err != nil {
		// Don't hard fail here as we should still attempt to delete the cluster
		log.Errorf("kind: failed to collect logs: %s", err)
	}

	log.Infof("kind: destroying cluster %q", clusterName)

	if err := DeleteCluster(clusterName); err != nil {
		return fmt.Errorf("failed to delete kind cluster: %s", err)
	}

	if err := os.Remove(k.KubeConfigPath()); err != nil {
		return fmt.Errorf("failed to delete kubeconfig file: %s", err)
	}

	log.Infof("kind: destroyed cluster %q", clusterName)

	return nil
}

func (k *Kind) collectLogs() error {
	provider := cluster.NewProvider()
	logDir := filepath.Join(k.rootPath, "artifacts", "logs")

	log.Infof("kind: collecting logs to %q", logDir)

	if err := os.RemoveAll(logDir); err != nil {
		return fmt.Errorf("failed to remove old logs directory: %s", err)
	}

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %s", err)
	}

	if err := provider.CollectLogs(clusterName, logDir); err != nil {
		return fmt.Errorf("failed to collect logs: %s", err)
	}

	log.Infof("kind: collected logs at %q", logDir)

	return nil
}

func (k *Kind) KubeClient() *kubernetes.Clientset {
	return k.client
}

func (k *Kind) KubeConfigPath() string {
	return filepath.Join(os.TempDir(), "kube-oidc-proxy-e2e")
}

func (k *Kind) Nodes() ([]nodes.Node, error) {
	return k.provider.ListNodes(clusterName)
}

func (k *Kind) errDestroy(err error) error {
	if dErr := k.Destroy(); dErr != nil {
		err = fmt.Errorf("%s\nkind failed to destroy: %s", err, dErr)
	}

	return err
}

func (k *Kind) waitForNodesReady() error {
	log.Infof("kind: waiting for all nodes to become ready...")

	return wait.PollImmediate(time.Second*5, time.Minute*10, func() (bool, error) {
		nodes, err := k.client.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		if len(nodes.Items) == 0 {
			log.Warn("kind: no nodes found - checking again...")
			return false, nil
		}

		var notReady []string
		for _, node := range nodes.Items {
			var ready bool
			for _, c := range node.Status.Conditions {
				if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
					ready = true
					break
				}
			}

			if !ready {
				notReady = append(notReady, node.Name)
			}
		}

		if len(notReady) > 0 {
			log.Infof("kind: nodes not ready: %s",
				strings.Join(notReady, ", "))
			return false, nil
		}

		return true, nil
	})
}

func (k *Kind) waitForCoreDNSReady() error {
	log.Infof("kind: waiting for all DNS pods to become ready...")
	return k.waitForPodsReady("kube-system", "k8s-app=kube-dns")
}

func (k *Kind) waitForPodsReady(namespace, labelSelector string) error {
	return wait.PollImmediate(time.Second*5, time.Minute*10, func() (bool, error) {
		pods, err := k.client.CoreV1().Pods(namespace).List(metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return false, err
		}

		if len(pods.Items) == 0 {
			log.Warnf("kind: no pods found in namespace %q with selector %q - checking again...",
				namespace, labelSelector)
			return false, nil
		}

		var notReady []string
		for _, pod := range pods.Items {
			if pod.Status.Phase != corev1.PodRunning {
				notReady = append(notReady, fmt.Sprintf("%s:%s (%s)",
					pod.Namespace, pod.Name, pod.Status.Phase))
			}
		}

		if len(notReady) > 0 {
			log.Infof("kind: pods not ready: %s",
				strings.Join(notReady, ", "))
			return false, nil
		}

		return true, nil
	})
}
