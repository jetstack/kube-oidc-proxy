// Copyright Jetstack Ltd. See LICENSE for details.
package utils

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"k8s.io/client-go/util/cert"
)

func NewTLSSelfSignedCertKey(dir, prefix string) (certPath, keyPath string, err error) {
	if prefix == "" {
		prefix = "kube-oidc-proxy"
	}

	cert, key, err := cert.GenerateSelfSignedCertKey("127.0.0.1", nil, []string{""})
	if err != nil {
		return "", "", err
	}

	certPath = filepath.Join(dir, fmt.Sprintf("%s-ca.pem", prefix))
	keyPath = filepath.Join(dir, fmt.Sprintf("%s-key.pem", prefix))

	err = ioutil.WriteFile(certPath, cert, 0600)
	if err != nil {
		return "", "", err
	}

	err = ioutil.WriteFile(keyPath, key, 0600)
	if err != nil {
		return "", "", err
	}

	return certPath, keyPath, nil
}
