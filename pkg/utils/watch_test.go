// Copyright Jetstack Ltd. See LICENSE for details.
package utils

import (
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/client-go/rest"
)

func Test_fileToWatchFromOptions(t *testing.T) {
	files, deferFunc, err := testFiles(t, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer deferFunc()

	retFiles := filesToWatchFromOptions(
		&rest.Config{
			BearerTokenFile: files[0],
			TLSClientConfig: rest.TLSClientConfig{
				CAFile:   files[1],
				CertFile: files[2],
				KeyFile:  files[3],
			},
		},

		&files[4],
		&files[5],

		&apiserveroptions.SecureServingOptions{
			ServerCert: apiserveroptions.GeneratableKeyCert{
				CertDirectory: "",
				PairName:      "pair-name",
				CertKey: apiserveroptions.CertKey{
					CertFile: files[6],
					KeyFile:  files[7],
				},
			},
		},
	)

	if !StringsEqual(files, retFiles) {
		t.Errorf("unexpected files to watch, exp=%s got=%s",
			files, retFiles)
	}

	tmpDir, err := ioutil.TempDir(os.TempDir(), "cert-pairs")
	if err != nil {
		t.Error(err)
		return
	}

	// create paired key-cert
	f, err := os.Create(filepath.Join(tmpDir, "pair-name.key"))
	if err != nil {
		t.Error(err)
		return
	}
	pairKey := f.Name()

	f, err = os.Create(filepath.Join(tmpDir, "pair-name.crt"))
	if err != nil {
		t.Error(err)
		return
	}
	pairCert := f.Name()

	// should still use overridden provided key pair
	retFiles = filesToWatchFromOptions(
		&rest.Config{
			BearerTokenFile: files[0],
			TLSClientConfig: rest.TLSClientConfig{
				CAFile:   files[1],
				CertFile: files[2],
				KeyFile:  files[3],
			},
		},

		&files[4],
		&files[5],

		&apiserveroptions.SecureServingOptions{
			ServerCert: apiserveroptions.GeneratableKeyCert{
				CertDirectory: tmpDir,
				PairName:      "pair-name",
				CertKey: apiserveroptions.CertKey{
					CertFile: files[6],
					KeyFile:  files[7],
				},
			},
		},
	)

	if !StringsEqual(files, retFiles) {
		t.Errorf("unexpected files to watch, exp=%s got=%s",
			files, retFiles)
	}

	// should not use certs in dir
	retFiles = filesToWatchFromOptions(
		&rest.Config{
			BearerTokenFile: files[0],
			TLSClientConfig: rest.TLSClientConfig{
				CAFile:   files[1],
				CertFile: files[2],
				KeyFile:  files[3],
			},
		},

		&files[4],
		&files[5],

		&apiserveroptions.SecureServingOptions{
			ServerCert: apiserveroptions.GeneratableKeyCert{
				CertDirectory: tmpDir,
				PairName:      "pair-name",
				CertKey: apiserveroptions.CertKey{
					CertFile: files[6],
				},
			},
		},
	)

	if !StringsEqual(append(files[:7]), retFiles) {
		t.Errorf("unexpected files to watch, exp=%s got=%s",
			files, retFiles)
	}

	// should not use certs in dir
	retFiles = filesToWatchFromOptions(
		&rest.Config{
			BearerTokenFile: files[0],
			TLSClientConfig: rest.TLSClientConfig{
				CAFile:   files[1],
				CertFile: files[2],
				KeyFile:  files[3],
			},
		},

		&files[4],
		&files[5],

		&apiserveroptions.SecureServingOptions{
			ServerCert: apiserveroptions.GeneratableKeyCert{
				CertDirectory: tmpDir,
				PairName:      "pair-name",
				CertKey: apiserveroptions.CertKey{
					KeyFile: files[6],
				},
			},
		},
	)

	if !StringsEqual(append(files[:7]), retFiles) {
		t.Errorf("unexpected files to watch, exp=%s got=%s",
			files, retFiles)
	}

	// should use certs in dir
	retFiles = filesToWatchFromOptions(
		&rest.Config{
			BearerTokenFile: files[0],
			TLSClientConfig: rest.TLSClientConfig{
				CAFile:   files[1],
				CertFile: files[2],
				KeyFile:  files[3],
			},
		},

		&files[4],
		&files[5],

		&apiserveroptions.SecureServingOptions{
			ServerCert: apiserveroptions.GeneratableKeyCert{
				CertDirectory: tmpDir,
				PairName:      "pair-name",
			},
		},
	)

	if !StringsEqual(append(files[:6], pairKey, pairCert), retFiles) {
		t.Errorf("unexpected files to watch, exp=%s got=%s",
			files, retFiles)
	}
}

func Test_watchFiles(t *testing.T) {
	files, deferFunc, err := testFiles(t, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer deferFunc()

	ch := make(chan os.Signal, 4)
	signal.Notify(ch, syscall.SIGHUP)

	for _, f := range files {
		watchFiles(time.Second/2, files)
		testWatchFileChange(t, f, ch)
	}
}

func testFiles(t *testing.T, count int) ([]string, func(), error) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "WatchSecretFiles")
	if err != nil {
		return nil, nil, err
	}
	deferFunc := func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("failed to remove temp directory: %s",
				err)
		}
	}

	files := make([]string, count)
	for i := range files {
		f, err := ioutil.TempFile(tmpDir, "")
		if err != nil {
			deferFunc()
			return nil, nil, err
		}

		files[i] = f.Name()
	}

	return files, deferFunc, nil
}

func testWatchFileChange(t *testing.T, f string, ch chan os.Signal) {
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
