// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Masterminds/semver"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/config"

	"github.com/jetstack/kube-oidc-proxy/pkg/utils"
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

	ctx := cluster.NewContext("kube-oidc-proxy-e2e")

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
	klog.Infof("creating kind cluster '%s'", ctx.Name())
	if err := ctx.Create(conf); err != nil {
		klog.Fatalf("error creating cluster: %s", err)
	}

	nodes, err := ctx.ListNodes()
	mustExit(err, "failed to list kind cluster nodes", tmpDir, ctx)

	if len(nodes) == 0 {
		mustExit(errors.New("no kind cluster nodes found"), "", tmpDir, ctx)
	}

	// copy kube service account public and private signing key to host
	for _, s := range []string{
		"sa.pub", "sa.key",
	} {
		err = nodes[0].CopyFrom("/etc/kubernetes/pki/"+s, filepath.Join(tmpDir, s))
		mustExit(err, "failed to copy file from node", tmpDir, ctx)
	}

	// generate rest config to kind cluster
	kubeconfig := ctx.KubeConfigPath()
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	mustExit(err, "failed to build kind rest client", tmpDir, ctx)

	kubeclient, err := kubernetes.NewForConfig(restConfig)
	mustExit(err, "failed to build kind kubernetes client", tmpDir, ctx)

	err = waitOnCoreDNS(kubeclient)
	mustExit(err, "failed to wait for CoreDNS to become ready", tmpDir, ctx)

	e2eSuite = New(kubeconfig, tmpDir, kubeclient)
	err = e2eSuite.Run()
	mustExit(err, "failed to start e2e suite", tmpDir, ctx)

	// run tests
	runErr := m.Run()

	// clean up and exit
	e2eSuite.cleanup()
	exitCode := cleanup(tmpDir, ctx, runErr)

	os.Exit(exitCode)
}

func mustExit(err error, errPrefix, tmpDir string, ctx *cluster.Context) {
	if err == nil {
		return
	}

	if len(errPrefix) > 0 {
		err = fmt.Errorf("%s: %s", errPrefix, err)
	}

	klog.Error(err)
	os.Exit(cleanup(tmpDir, ctx, 1))
}

func cleanup(tmpDir string, ctx *cluster.Context, exitCode int) int {
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
			fmt.Sprintf("--kubeconfig=%s", ctx.KubeConfigPath()),
		)
		exportOut, err := exportCmd.Output()
		if err != nil {
			klog.Errorf("failed to export events: %s", err)
		} else {
			klog.Infof("exported events from failed e2e tests:\n%s",
				exportOut)
		}
	}

	klog.Infof("destroying kind cluster '%s'", ctx.Name())
	kindErr := ctx.Delete()
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
		err := utils.WaitForPodReady(kubeclient, pod.Name, pod.Namespace)
		if err != nil {
			return fmt.Errorf("failed to wait for dns pods to become ready: %s", err)
		}
	}

	return nil
}
