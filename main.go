package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"

	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/apiserver/plugin/pkg/authenticator/token/oidc"
)

type Proxy struct {
	//req authenticator.Request
}

func main() {
	cert, err := tls.LoadX509KeyPair("client.crt", "client.key")
	if err != nil {
		log.Fatal(err)
	}

	// Load CA cert
	caCert, err := ioutil.ReadFile("client.ca")
	if err != nil {
		log.Fatal(err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// Setup HTTPS client
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}
	tlsConfig.BuildNameToCertificate()
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: transport}

	config := oidc.Options{
		IssuerURL:     "https://accounts.google.com",
		ClientID:      "",
		UsernameClaim: "sub",
	}

	iodcAuther, err := oidc.New(config)
	if err != nil {
		log.Fatal(err)
	}
	reqAuther := bearertoken.New(iodcAuther)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, ok, err := reqAuther.AuthenticateRequest(r)
		if err != nil {
			log.Printf("Unable to authenticate the request due to an error: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized"))
			return
		}
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized"))
			return
		}

		URL, err := url.Parse("https://api.jvl-cluster.develop.tarmak.org" + r.URL.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse error: "+err.Error()+"\n")
			return
		}

		req := &http.Request{
			URL:  URL,
			Body: r.Body,
		}
		req.Header = r.Header

		res, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "do error: "+err.Error()+"\n")
			return
		}

		setHeader(w, res.Header)
		w.WriteHeader(res.StatusCode)
		io.Copy(w, res.Body)
		res.Body.Close()
	})

	err = http.ListenAndServeTLS(":8000", "apiserver.crt", "apiserver.key", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func setHeader(w http.ResponseWriter, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
}
