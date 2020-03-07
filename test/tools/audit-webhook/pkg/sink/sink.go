// Copyright Jetstack Ltd. See LICENSE for details.
package sink

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"sync"

	log "github.com/sirupsen/logrus"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

type Sink struct {
	logPath           string
	keyFile, certFile string

	sk *rsa.PrivateKey

	stopCh <-chan struct{}
	mu     sync.Mutex
}

func New(logPath, keyFile, certFile string, stopCh <-chan struct{}) (*Sink, error) {
	b, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(b)
	if block == nil {
		return nil,
			fmt.Errorf("failed to parse PEM block containing the key: %q", keyFile)
	}

	sk, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return &Sink{
		logPath:  logPath,
		keyFile:  keyFile,
		certFile: certFile,
		sk:       sk,
		stopCh:   stopCh,
	}, nil
}

func (s *Sink) Run(bindAddress, listenPort string) (<-chan struct{}, error) {
	serveAddr := fmt.Sprintf("%s:%s", bindAddress, listenPort)

	l, err := net.Listen("tcp", serveAddr)
	if err != nil {
		return nil, err
	}

	go func() {
		<-s.stopCh
		if l != nil {
			l.Close()
		}
	}()

	compCh := make(chan struct{})
	go func() {
		defer close(compCh)

		err := http.ServeTLS(l, s, s.certFile, s.keyFile)
		if err != nil {
			log.Errorf("stopped serving TLS (%s): %s", serveAddr, err)
		}
	}()

	log.Infof("audit webhook listening and serving on %s", serveAddr)

	return compCh, nil
}

func (s *Sink) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	log.Infof("%s: audit webhook received url %s", r.RemoteAddr, r.URL)

	var events auditv1.EventList
	err := json.NewDecoder(r.Body).Decode(&events)
	if err != nil {
		log.Errorf("%s: failed to decode request body: %s", r.RemoteAddr, err)
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	log.Infof("%s: got events: %v", r.RemoteAddr, events)

	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(s.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Errorf("%s: failed to open log file: %s", r.RemoteAddr, err)
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	defer f.Close()

	for _, event := range events.Items {
		if err := json.NewEncoder(f).Encode(event); err != nil {
			log.Errorf("%s: failed to write audit event: %s", r.RemoteAddr, err)
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
	}

	rw.WriteHeader(http.StatusOK)
}
