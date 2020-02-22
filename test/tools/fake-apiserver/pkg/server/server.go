// Copyright Jetstack Ltd. See LICENSE for details.
package server

import (
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	log "github.com/sirupsen/logrus"
)

type Server struct {
	keyFile, certFile string

	stopCh <-chan struct{}
}

func New(keyFile, certFile string, stopCh <-chan struct{}) (*Server, error) {
	b, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(b)
	if block == nil {
		return nil,
			fmt.Errorf("failed to parse PEM block containing the key: %q", keyFile)
	}

	return &Server{
		keyFile:  keyFile,
		certFile: certFile,
		stopCh:   stopCh,
	}, nil
}

func (s *Server) Run(bindAddress, listenPort string) (<-chan struct{}, error) {
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

	log.Infof("fake API server listening and serving on %s", serveAddr)

	return compCh, nil
}

func (s *Server) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	log.Infof("(%s) Fake API server received url %s", r.URL, r.RemoteAddr)

	log.Infof("(%s) Request headers:", r.RemoteAddr)
	for k, vs := range r.Header {
		for _, v := range vs {
			log.Infof("(%s) %s: %s", r.RemoteAddr, k, v)
			rw.Header().Add(k, v)
		}
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Errorf("failed to read request body: %s", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Infof("(%s) Request Body: %s", r.RemoteAddr, body)

	if _, err := rw.Write(body); err != nil {
		log.Errorf("failed to write request body to response: %s", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	rw.WriteHeader(http.StatusOK)
}
