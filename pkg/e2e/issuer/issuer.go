// Copyright Jetstack Ltd. See LICENSE for details.
package issuer

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"path/filepath"
	"time"

	"k8s.io/klog"
)

type Issuer struct {
	tmpDir            string
	listenPort        string
	certPath, keyPath string
}

func New(tmpDir, listenPort string) *Issuer {
	return &Issuer{
		tmpDir:     tmpDir,
		listenPort: listenPort,
	}
}

func (i *Issuer) Run() error {
	key, cert, err := i.genSelfSignedCert()
	if err != nil {
		return fmt.Errorf("failed to generate self signed certificates: %s", err)
	}

	i.certPath = filepath.Join(i.tmpDir, "oidc-issuer-ca.pem")
	i.keyPath = filepath.Join(i.tmpDir, "oidc-issuer-key.pem")

	err = ioutil.WriteFile(i.certPath, cert, 0600)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(i.keyPath, key, 0600)
	if err != nil {
		return err
	}

	serveAddr := fmt.Sprintf("127.0.0.1:%s", i.listenPort)

	go func() {
		err = http.ListenAndServeTLS(serveAddr, i.certPath, i.keyPath, i)
		if err != nil {
			klog.Errorf("failed to server secure tls: %s", err)
		}
	}()

	time.Sleep(time.Second * 2)

	return nil
}

func (i *Issuer) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
}

func (i *Issuer) CertPath() string {
	return i.certPath
}

func (i *Issuer) KeyPath() string {
	return i.keyPath
}

func (i *Issuer) genSelfSignedCert() ([]byte, []byte, error) {
	sk, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	tmpl := x509.Certificate{
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24 * 180),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		SerialNumber:          big.NewInt(1),
	}

	cert, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, sk.Public(), sk)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %s", err)
	}

	certPem := &bytes.Buffer{}
	keyPem := &bytes.Buffer{}

	err = pem.Encode(certPem, &pem.Block{Type: "CERTIFICATE", Bytes: cert})
	if err != nil {
		return nil, nil, err
	}

	err = pem.Encode(keyPem, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(sk)})
	if err != nil {
		return nil, nil, err
	}

	return keyPem.Bytes(), certPem.Bytes(), nil
}
