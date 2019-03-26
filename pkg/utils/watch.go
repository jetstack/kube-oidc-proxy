// Copyright Jetstack Ltd. See LICENSE for details.
package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

type fileWatched struct {
	name    string
	modTime time.Time
}

func WatchSecretFiles(restConfig *rest.Config,
	oidcCAFile *string, kubeconfig *string,
	ssoptions *apiserveroptions.SecureServingOptions,
	refreshTimer time.Duration) error {

	files := filesToWatchFromOptions(
		restConfig, oidcCAFile, kubeconfig, ssoptions)

	if err := watchFiles(refreshTimer, files); err != nil {
		return err
	}

	klog.Infof("watching for changes in files %s", files)

	return nil
}

func filesToWatchFromOptions(restConfig *rest.Config,
	oidcCAFile *string, kubeconfig *string,
	ssoptions *apiserveroptions.SecureServingOptions,
) []string {
	var watchFiles []string

	if len(restConfig.BearerTokenFile) > 0 {
		watchFiles = append(watchFiles, restConfig.BearerTokenFile)
	}
	if len(restConfig.CAFile) > 0 {
		watchFiles = append(watchFiles, restConfig.CAFile)
	}
	if len(restConfig.CertFile) > 0 {
		watchFiles = append(watchFiles, restConfig.CertFile)
	}
	if len(restConfig.KeyFile) > 0 {
		watchFiles = append(watchFiles, restConfig.KeyFile)
	}

	if kubeconfig != nil && len(*kubeconfig) > 0 {
		watchFiles = append(watchFiles, *kubeconfig)
	}

	if oidcCAFile != nil && len(*oidcCAFile) > 0 {
		watchFiles = append(watchFiles, *oidcCAFile)
	}

	for _, sni := range ssoptions.SNICertKeys {
		if len(sni.KeyFile) > 0 {
			watchFiles = append(watchFiles, sni.KeyFile)
		}

		if len(sni.CertFile) > 0 {
			watchFiles = append(watchFiles, sni.CertFile)
		}
	}

	// watch cert directory if key and cert file not explicitly given
	if len(ssoptions.ServerCert.CertKey.CertFile) == 0 &&
		len(ssoptions.ServerCert.CertKey.KeyFile) == 0 &&
		len(ssoptions.ServerCert.CertDirectory) > 0 {

		watchFiles = append(watchFiles,
			filepath.Join(ssoptions.ServerCert.CertDirectory,
				ssoptions.ServerCert.PairName+".crt"))

		watchFiles = append(watchFiles,
			filepath.Join(ssoptions.ServerCert.CertDirectory,
				ssoptions.ServerCert.PairName+".key"))

	} else {

		if len(ssoptions.ServerCert.CertKey.CertFile) > 0 {
			watchFiles = append(watchFiles,
				ssoptions.ServerCert.CertKey.CertFile)
		}
		if len(ssoptions.ServerCert.CertKey.KeyFile) > 0 {
			watchFiles = append(watchFiles,
				ssoptions.ServerCert.CertKey.KeyFile)
		}
	}

	return watchFiles
}

func watchFiles(refreshTimer time.Duration, files []string) error {

	// initialise file modtimes
	var watched []fileWatched
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			return fmt.Errorf(
				"failed to get file info of %s to watch: %s", f, err)
		}

		watched = append(watched,
			fileWatched{f, info.ModTime()})
	}

	// loop, waiting for change in a file
	// send SIGHUP to self once one has been detected
	go func() {
		for {
			time.Sleep(refreshTimer)

			for _, f := range watched {
				info, err := os.Stat(f.name)
				if err != nil {
					klog.Errorf("failed to get file stat %s: %s",
						f.name, err)
					continue
				}

				// file has been updated
				if info.ModTime().After(f.modTime) {
					klog.Infof("detected change in file %s, exiting", f.name)

					// find self process
					p, err := os.FindProcess(os.Getpid())
					if err != nil {
						klog.Errorf("failed to get current pid: %s", err)
						continue
					}

					// send SIGHUP to self
					if err := p.Signal(syscall.SIGHUP); err != nil {
						klog.Errorf("failed to signal current process: %s", err)
						continue
					}

					// SIGHUP successful, exit routine
					return
				}
			}
		}
	}()

	return nil
}
