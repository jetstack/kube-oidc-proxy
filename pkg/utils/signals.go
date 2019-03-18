// Copyright Jetstack Ltd. See LICENSE for details.
package utils

import (
	"os"
	"os/signal"
	"syscall"

	"k8s.io/klog"
)

func SignalHandler() chan struct{} {
	stopCh := make(chan struct{})
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-ch

		close(stopCh)

		for i := 0; i < 3; i++ {
			klog.V(0).Infof("received signal %s, shutting down gracefully...", sig)
			sig = <-ch
		}

		klog.V(0).Infof("received signal %s, force closing", sig)

		os.Exit(1)
	}()

	return stopCh
}
