// Copyright Jetstack Ltd. See LICENSE for details.
package util

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"k8s.io/client-go/util/cert"
)

type KeyCertPair struct {
	CertPath string
	KeyPath  string
	Cert     []byte
	Key      *rsa.PrivateKey
}

func NewTLSSelfSignedCertKey(dir, prefix string) (*KeyCertPair, error) {
	if prefix == "" {
		prefix = "kube-oidc-proxy"
	}

	certBytes, keyBytes, err := cert.GenerateSelfSignedCertKey("127.0.0.1", nil, []string{""})
	if err != nil {
		return nil, err
	}

	certPath := filepath.Join(dir, fmt.Sprintf("%s-ca.pem", prefix))
	keyPath := filepath.Join(dir, fmt.Sprintf("%s-key.pem", prefix))

	err = ioutil.WriteFile(certPath, certBytes, 0600)
	if err != nil {
		return nil, err
	}

	err = ioutil.WriteFile(keyPath, keyBytes, 0600)
	if err != nil {
		return nil, err
	}

	p, rest := pem.Decode(keyBytes)
	if len(rest) != 0 {
		return nil, fmt.Errorf("got rest decoding pem file %s: %s", keyPath, rest)
	}

	sk, err := x509.ParsePKCS1PrivateKey(p.Bytes)
	if err != nil {
		return nil, err
	}

	return &KeyCertPair{
		CertPath: certPath,
		KeyPath:  keyPath,
		Cert:     certBytes,
		Key:      sk,
	}, nil
}
