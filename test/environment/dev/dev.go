// Copyright Jetstack Ltd. See LICENSE for details.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"

	"github.com/jetstack/kube-oidc-proxy/test/e2e/framework/config"
	"github.com/jetstack/kube-oidc-proxy/test/e2e/framework/helper"
	"github.com/jetstack/kube-oidc-proxy/test/environment"
	"github.com/jetstack/kube-oidc-proxy/test/kind"
)

const (
	clientID = "kube-oidc-proxy-e2e-client-id"
)

func main() {
	if len(os.Args) != 2 {
		errExit(fmt.Errorf("expecting 2 arguments, got=%d",
			len(os.Args)))
	}

	switch os.Args[1] {
	case "create":
		create()
	case "deploy":
		deploy()
	case "destroy":
		destroy()
	default:
		errExit(fmt.Errorf("unexpected argument %q, expecting [%q %q %q]",
			os.Args[1], "create", "deploy", "destroy"))
	}

	os.Exit(0)
}

func create() {
	env, err := environment.Create(1, 1)
	errExit(err)

	fmt.Printf("> dev environment created.\n")
	fmt.Printf("export KUBECONFIG=%s\n", env.KubeConfigPath())
}

func deploy() {
	k := new(kind.Kind)
	kubeconfig := k.KubeConfigPath()
	rootPath, err := environment.RootPath()
	errExit(err)

	cfg := &config.Config{
		KubeConfigPath: kubeconfig,
		RepoRoot:       rootPath,
		Kubectl:        filepath.Join(rootPath, "bin", "kubectl"),
	}

	err = cfg.Validate()
	errExit(err)

	helper := helper.NewHelper(cfg)

	clientConfigFlags := genericclioptions.NewConfigFlags(true)
	clientConfigFlags.KubeConfig = &kubeconfig
	config, err := clientConfigFlags.ToRESTConfig()
	errExit(err)

	kubeClient, err := kubernetes.NewForConfig(config)
	errExit(err)

	helper.KubeClient = kubeClient

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "kube-oidc-proxy-e2e-",
		},
	}
	ns, err = kubeClient.CoreV1().Namespaces().Create(ns)
	errExit(err)

	fmt.Printf("> created new namespace %s\n", ns.Name)

	issuerKeyBundle, issuerURL, err := helper.DeployIssuer(ns.Name)
	errExit(err)

	fmt.Printf("> deployed issuer at url %s\n", issuerURL)

	_, proxyURL, err := helper.DeployProxy(ns, issuerURL,
		"kube-oidc-proxy-e2e-client-id", issuerKeyBundle)
	errExit(err)
	fmt.Printf("> deployed proxy at url %s\n", proxyURL)

	tokenPayload := helper.NewTokenPayload(issuerURL, clientID, time.Now().Add(time.Hour*48))

	signedToken, err := helper.SignToken(issuerKeyBundle, tokenPayload)
	errExit(err)
	fmt.Printf("> signed token valid for 48 hours:\n%s\n", signedToken)

	fmt.Printf("export KUBECONFIG=%s\n", kubeconfig)
}

func destroy() {
	errExit(kind.DeleteCluster("kube-oidc-proxy-e2e"))
	fmt.Printf("> dev environment destroyed.\n")
}

func errExit(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
