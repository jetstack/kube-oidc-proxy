// Copyright Jetstack Ltd. See LICENSE for details.
package main

import (
	"fmt"
	"os"

	"github.com/jetstack/kube-oidc-proxy/test/environment"
	"github.com/jetstack/kube-oidc-proxy/test/kind"
)

func main() {
	if len(os.Args) != 2 {
		errExit(fmt.Errorf("expecting 2 arguments, got=%d",
			len(os.Args)))
	}

	switch os.Args[1] {
	case "create":
		create()
	case "destroy":
		destroy()
	default:
		errExit(fmt.Errorf("unexpected argument %q, expecting %q or %q",
			os.Args[1], "create", "destroy"))
	}

	os.Exit(0)
}

func create() {
	env, err := environment.Create(1, 1)
	errExit(err)

	fmt.Printf("dev environment created.\n")
	kubeconfig, err := env.KubeConfigPath()
	if err != nil {
		fmt.Printf("failed to determine kubeconfig path: %s", err)
		return
	}

	fmt.Printf("export KUBECONFIG=%s", kubeconfig)
}

func destroy() {
	errExit(kind.DeleteCluster("kube-oidc-proxy-e2e"))
	fmt.Printf("dev environment destroyed.\n")
}

func errExit(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
