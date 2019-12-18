// Copyright Jetstack Ltd. See LICENSE for details.
package kind

import (
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	configv1alpha3 "sigs.k8s.io/kind/pkg/apis/config/v1alpha3"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/create"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
)

type Kind struct {
	rootPath string

	ctx        *cluster.Context
	restConfig *rest.Config
	client     *kubernetes.Clientset
}

func New(rootPath, nodeImage string, masterNodes, workerNodes int) (*Kind, error) {
	log.Infof("kind: using k8s node image %q", nodeImage)

	k := &Kind{
		rootPath: rootPath,
		ctx:      cluster.NewContext("kube-oidc-proxy-e2e"),
	}

	conf := new(configv1alpha3.Cluster)
	configv1alpha3.SetDefaults_Cluster(conf)
	conf.Nodes = nil

	// This behviour will be changing soon in later versions of kind.
	if len(workingNodes) == 0 {
		for i := 0; i < masterNodes; i++ {
			conf.Nodes = append(conf.Nodes,
				configv1alpha3.Node{
					Image: nodeImage,
				})
		}

	} else {
		for i := 0; i < masterNodes; i++ {
			conf.Nodes = append(conf.Nodes,
				configv1alpha3.Node{
					Image: nodeImage,
					Role:  configv1alpha3.ControlPlaneRole,
				})
		}

		for i := 0; i < workerNodes; i++ {
			conf.Nodes = append(conf.Nodes,
				configv1alpha3.Node{
					Image: nodeImage,
					Role:  configv1alpha3.WorkerRole,
				})
		}
	}

	conf.Networking.ServiceSubnet = "10.0.0.0/16"

	// create kind cluster
	log.Infof("kind: creating kind cluster %q", k.ctx.Name())
	if err := k.ctx.Create(create.WithV1Alpha3(conf)); err != nil {
		return nil, fmt.Errorf("failed to create cluster: %s", err)
	}

	// generate rest config to kind cluster
	kubeconfig := k.ctx.KubeConfigPath()
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
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

	log.Infof("kind: cluster ready %q", k.ctx.Name())

	return k, nil
}

func DeleteCluster(name string) error {
	ok, err := cluster.IsKnown(name)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("cluster unknown: %q", name)
	}

	return cluster.NewContext(name).Delete()
}

func (k *Kind) Destroy() error {
	log.Infof("kind: destroying cluster %q", k.ctx.Name())
	if err := k.ctx.Delete(); err != nil {
		return fmt.Errorf("failed to delete kind cluster: %s", err)
	}

	log.Infof("kind: destroyed cluster %q", k.ctx.Name())

	return nil
}

func (k *Kind) KubeClient() *kubernetes.Clientset {
	return k.client
}

func (k *Kind) KubeConfigPath() string {
	return k.ctx.KubeConfigPath()
}

func (k *Kind) Nodes() ([]nodes.Node, error) {
	return k.ctx.ListNodes()
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
