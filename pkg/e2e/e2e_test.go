// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/Masterminds/semver"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/create"
	config "sigs.k8s.io/kind/pkg/apis/config/v1alpha3"

	"github.com/jetstack/kube-oidc-proxy/pkg/util"
)

const (
	defaultNodeImage = "1.15.0"
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

	v13, err := semver.NewVersion("1.13")
	if err != nil {
		klog.Fatal(err)
	}

	v12, err := semver.NewVersion("1.12")
	if err != nil {
		klog.Fatal(err)
	}

	v, err := semver.NewVersion(nodeImage)
	if err != nil {
		klog.Fatalf("failed to parse not image version %s: %s",
			nodeImage, err)
	}

	nodeImage = fmt.Sprintf("kindest/node:v%s", nodeImage)

	clusterContext := cluster.NewContext("kube-oidc-proxy-e2e")

	// build default config
	conf := new(config.Cluster)
	config.SetDefaults_Cluster(conf)
	if len(conf.Nodes) == 0 {
		klog.Fatal("kind default config set node count to 0")
	}

	if v.Compare(v13) < 0 {
		kubeadmConfig := `metadata:
  name: config
networking:
  serviceSubnet: 10.0.0.0/16`

		if v.Compare(v12) < 0 {
			kubeadmConfig = fmt.Sprint(`apiVersion: kubeadm.k8s.io/v1alpha2
kind: MasterConfiguration
`, kubeadmConfig)
		} else {
			kubeadmConfig = fmt.Sprint(`apiVersion: kubeadm.k8s.io/v1alpha3
kind: ClusterConfiguration
`, kubeadmConfig)
		}

		conf.KubeadmConfigPatches = []string{kubeadmConfig}
	} else {
		conf.Networking.ServiceSubnet = "10.0.0.0/16"
	}

	for i := range conf.Nodes {
		conf.Nodes[i].Image = nodeImage
	}

	log.SetLevel(log.DebugLevel)

	// create kind cluster
	klog.Infof("creating kind cluster '%s'", clusterContext.Name())
	if err := clusterContext.Create(create.WithV1Alpha3(conf)); err != nil {
		klog.Fatalf("error creating cluster: %s", err)
	}

	// generate rest config to kind cluster
	kubeconfig := clusterContext.KubeConfigPath()
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Errorf("failed to build kind rest client: %s", err)
		os.Exit(cleanup(tmpDir, clusterContext, 1))
	}

	kubeclient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("failed to build kind kubernetes client: %s", err)
		os.Exit(cleanup(tmpDir, clusterContext, 1))
	}

	if err := waitOnCoreDNS(kubeclient); err != nil {
		klog.Errorf("failed to wait for CoreDNS to become ready: %s", err)
		os.Exit(cleanup(tmpDir, clusterContext, 1))
	}

	e2eSuite = New(kubeconfig, tmpDir, kubeclient)
	if err := e2eSuite.Run(); err != nil {
		klog.Errorf("failed to start e2e suite: %s", err)
		os.Exit(cleanup(tmpDir, clusterContext, 1))
	}

	// run tests
	runErr := m.Run()

	// clean up and exit
	e2eSuite.cleanup()
	exitCode := cleanup(tmpDir, clusterContext, runErr)

	os.Exit(exitCode)
}

func cleanup(tmpDir string, clusterContext *cluster.Context, exitCode int) int {
	err := os.RemoveAll(tmpDir)
	if err != nil {
		klog.Errorf("failed to delete temp dir %s: %s", tmpDir, err)
	}

	if exitCode != 0 {
		exportCmd := exec.Command(
			"../../bin/kubectl",
			"get",
			"events",
			"--all-namespaces",
			fmt.Sprintf("--kubeconfig=%s", clusterContext.KubeConfigPath()),
		)
		exportOut, err := exportCmd.Output()
		if err != nil {
			klog.Errorf("failed to export events: %s", err)
		} else {
			klog.Infof("exported events from failed e2e tests:\n%s",
				exportOut)
		}
	}

	klog.Infof("destroying kind cluster '%s'", clusterContext.Name())
	kindErr := clusterContext.Delete()
	if kindErr != nil {
		klog.Errorf("error destroying kind cluster: %s", err)
	}

	if err != nil || kindErr != nil {
		return 1
	}

	return exitCode
}

func waitOnCoreDNS(kubeclient *kubernetes.Clientset) error {
	// ensure pods are deployed
	time.Sleep(time.Second * 15)

	pods, err := kubeclient.CoreV1().Pods("kube-system").List(metav1.ListOptions{
		LabelSelector: "k8s-app=kube-dns",
	})
	if err != nil {
		return err
	}

	if len(pods.Items) == 0 {
		return errors.New("failed to find pods with label k8s-app=kube-dns")
	}

	for _, pod := range pods.Items {
		err := util.WaitForPodReady(kubeclient, pod.Name, pod.Namespace)
		if err != nil {
			return fmt.Errorf("failed to wait for dns pods to become ready: %s", err)
		}
	}

	return nil
}
