// Copyright Jetstack Ltd. See LICENSE for details.
package kind

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
)

func (k *Kind) LoadAllImages() error {
	if err := k.LoadKubeOIDCProxy(); err != nil {
		return err
	}

	if err := k.LoadIssuer(); err != nil {
		return err
	}

	if err := k.LoadFakeAPIServer(); err != nil {
		return err
	}

	if err := k.LoadAuditWebhook(); err != nil {
		return err
	}

	return nil
}

func (k *Kind) LoadKubeOIDCProxy() error {
	binPath := filepath.Join(k.rootPath, "./bin/kube-oidc-proxy-linux")
	mainPath := filepath.Join(k.rootPath, "./cmd/.")
	image := "kube-oidc-proxy-e2e"

	return k.loadImage(binPath, mainPath, image, k.rootPath)
}

func (k *Kind) LoadIssuer() error {
	binPath := filepath.Join(k.rootPath, "./test/tools/issuer/bin/oidc-issuer-linux")
	dockerfilePath := filepath.Join(k.rootPath, "./test/tools/issuer")
	mainPath := filepath.Join(dockerfilePath, "cmd")
	image := "oidc-issuer-e2e"

	return k.loadImage(binPath, mainPath, image, dockerfilePath)
}

func (k *Kind) LoadFakeAPIServer() error {
	binPath := filepath.Join(k.rootPath, "./test/tools/fake-apiserver/bin/fake-apiserver-linux")
	dockerfilePath := filepath.Join(k.rootPath, "./test/tools/fake-apiserver")
	mainPath := filepath.Join(dockerfilePath, "cmd")
	image := "fake-apiserver-e2e"

	return k.loadImage(binPath, mainPath, image, dockerfilePath)
}

func (k *Kind) LoadAuditWebhook() error {
	binPath := filepath.Join(k.rootPath, "./test/tools/audit-webhook/bin/audit-webhook")
	dockerfilePath := filepath.Join(k.rootPath, "./test/tools/audit-webhook")
	mainPath := filepath.Join(dockerfilePath, "cmd")
	image := "audit-webhook-e2e"

	return k.loadImage(binPath, mainPath, image, dockerfilePath)
}

func (k *Kind) loadImage(binPath, mainPath, image, dockerfilePath string) error {
	log.Infof("kind: building %q", mainPath)

	if err := os.MkdirAll(filepath.Dir(binPath), 0755); err != nil {
		return err
	}

	err := k.runCmd("go", "build", "-v", "-o", binPath, mainPath)
	if err != nil {
		return err
	}

	err = k.runCmd("docker", "build", "-t", image, dockerfilePath)
	if err != nil {
		return err
	}

	tmpDir, err := ioutil.TempDir(os.TempDir(), "kube-oidc-proxy-e2e")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	imageArchive := filepath.Join(tmpDir, fmt.Sprintf("%s-e2e.tar", image))
	log.Infof("kind: saving image to archive %q", imageArchive)

	err = k.runCmd("docker", "save", "--output="+imageArchive, image)
	if err != nil {
		return err
	}

	nodes, err := k.Nodes()
	if err != nil {
		return err
	}

	b, err := ioutil.ReadFile(imageArchive)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		log.Infof("kind: loading image %q to node %q", image, node.String())
		r := bytes.NewBuffer(b)
		if err := nodeutils.LoadImageArchive(node, r); err != nil {
			return err
		}

		err := node.Command("mkdir", "-p", "/tmp/kube-oidc-proxy").Run()
		if err != nil {
			return fmt.Errorf("failed to create directory %q: %s",
				"/tmp/kube-oidc-proxy", err)
		}
	}

	return nil
}

func (k *Kind) runCmd(command string, args ...string) error {
	return k.runCmdWithOut(os.Stdout, command, args...)
}

func (k *Kind) runCmdWithOut(w io.Writer, command string, args ...string) error {
	log.Infof("kind: running command '%s %s'", command, strings.Join(args, " "))
	cmd := exec.Command(command, args...)

	cmd.Stderr = os.Stderr
	cmd.Stdout = w
	cmd.Env = append(cmd.Env,
		"GO111MODULE=on", "CGO_ENABLED=0", "HOME="+os.Getenv("HOME"),
		"PATH="+os.Getenv("PATH"),
		"GOARCH=amd64", "GOOS=linux")

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}
