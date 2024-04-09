// Copyright Jetstack Ltd. See LICENSE for details.
package main

import (
	"fmt"
	"os"

	"github.com/jetstack/kube-oidc-proxy/cmd/app"
	"github.com/jetstack/kube-oidc-proxy/pkg/util"
	"k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(nil)
	stopCh := util.SignalHandler()
	cmd := app.NewRunCommand(stopCh)

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
