// Copyright Jetstack Ltd. See LICENSE for details.
package utils

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"k8s.io/client-go/util/cert"
)

func NewTLSSelfSignedCertKey(dir, prefix string) (certPath, keyPath string, sk *rsa.PrivateKey, certBytes []byte, err error) {
	if prefix == "" {
		prefix = "kube-oidc-proxy"
	}

	certBytes, keyBytes, err := cert.GenerateSelfSignedCertKey("127.0.0.1", nil, []string{""})
	if err != nil {
		return "", "", nil, nil, err
	}

	certPath = filepath.Join(dir, fmt.Sprintf("%s-ca.pem", prefix))
	keyPath = filepath.Join(dir, fmt.Sprintf("%s-key.pem", prefix))

	err = ioutil.WriteFile(certPath, certBytes, 0600)
	if err != nil {
		return "", "", nil, nil, err
	}

	err = ioutil.WriteFile(keyPath, keyBytes, 0600)
	if err != nil {
		return "", "", nil, nil, err
	}

	p, rest := pem.Decode(keyBytes)
	if len(rest) != 0 {
		return "", "", nil, nil,
			fmt.Errorf("got rest decoding pem file %s: %s", keyPath, rest)
	}

	sk, err = x509.ParsePKCS1PrivateKey(p.Bytes)
	if err != nil {
		return "", "", nil, nil, err
	}

	return certPath, keyPath, sk, certBytes, nil
}
