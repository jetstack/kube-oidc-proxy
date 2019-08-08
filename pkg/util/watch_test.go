// Copyright Jetstack Ltd. See LICENSE for details.
package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestWatchFiles(t *testing.T) {
	files, dir, cleanupF, err := genTestFiles(t, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanupF()

	syms := make([]string, 2)

	// should also work through symbolic links
	for i := range syms {
		symDir, err := ioutil.TempDir(dir, "WatchSecretFiles")
		if err != nil {
			t.Error(err)
			return
		}

		syms[i] = filepath.Join(symDir, fmt.Sprintf("symlink-%d", i))
		if err := os.Symlink(files[i+1], syms[i]); err != nil {
			t.Errorf("failed to create symlink: %s", err)
		}
	}

	ch := make(chan os.Signal, 4)
	signal.Notify(ch, syscall.SIGHUP)

	for _, f := range files {
		err := WatchFiles(time.Second/4,
			// mix of actual paths and symbolic links
			[]string{files[0], syms[0], syms[1], files[3]})
		if err != nil {
			t.Fatal(err)
		}

		expectWatchFileChange(t, f, ch)
	}
}

func genTestFiles(t *testing.T, count int) ([]string, string, func(), error) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "WatchSecretFiles")
	if err != nil {
		return nil, "", nil, err
	}
	cleanupF := func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("failed to remove temp directory: %s",
				err)
		}
	}

	files := make([]string, count)
	for i := range files {
		f, err := ioutil.TempFile(tmpDir, "")
		if err != nil {
			cleanupF()
			return nil, "", nil, err
		}

		files[i] = f.Name()
	}

	return files, tmpDir, cleanupF, nil
}

func expectWatchFileChange(t *testing.T, f string, ch chan os.Signal) {
	ticker := time.NewTicker(time.Second)

	// no change
	select {
	case <-ch:
		t.Errorf(
			"expected no HUP signal after not changing file %s", f)
	case <-ticker.C:
		break
	}

	// change
	err := ioutil.WriteFile(f, []byte("a"), 0644)
	if err != nil {
		t.Errorf("failed to write to watch file: %s", err)
		return
	}

	select {
	case <-ch:
		break
	case <-ticker.C:
		t.Errorf(
			"expected HUP signal after changing file %s", f)
	}

	// change again
	err = ioutil.WriteFile(f, []byte("b"), 0644)
	if err != nil {
		t.Errorf("failed to write to watch file: %s", err)
		return
	}

	select {
	case <-ch:
		t.Errorf(
			"expected no HUP signal after changing file twice %s", f)
	case <-ticker.C:
		break
	}
}
