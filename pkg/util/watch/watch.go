// Copyright Jetstack Ltd. See LICENSE for details.
package watch

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"k8s.io/klog"
)

type fileWatched struct {
	name    string
	modTime time.Time
}

func Files(refreshTimer time.Duration, files []string) error {
	if len(files) == 0 {
		return nil
	}

	var errs []string

	// initialise file modtimes
	var watched []fileWatched
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			errs = append(errs,
				fmt.Sprintf("failed to get file info of %s to watch: %s", f, err))
			continue
		}

		watched = append(watched,
			fileWatched{f, info.ModTime()})
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to start file watchers: %s",
			strings.Join(errs, ","))
	}

	klog.Infof("watching files %s", files)

	ticker := time.NewTicker(refreshTimer)

	// loop, waiting for change in a file
	// send SIGHUP to self once one has been detected
	go func() {
		for {
			<-ticker.C
			if exitOnChange(watched) {
				return
			}
		}
	}()

	return nil
}

func exitOnChange(fw []fileWatched) bool {
	for _, f := range fw {
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
				return false
			}

			// send SIGHUP to self
			if err := p.Signal(syscall.SIGHUP); err != nil {
				klog.Errorf("failed to signal current process: %s", err)
				return false
			}

			return true
		}
	}

	return false
}
