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
	//fs := flag.NewFlagSet("kube-oidc-proxy", flag.ContinueOnError)
	//klog.InitFlags(fs)

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
		info, ok, err := reqAuther.AuthenticateRequest(r)
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

		fmt.Printf("--------\n")
		fmt.Printf("INFO: %s\n", info)
		fmt.Printf("ERR: %s\n", err)
		fmt.Printf("OK: %t\n", ok)
		fmt.Printf("%s\n", r.URL.Path)
		fmt.Printf(">%+v\n", r.Header)

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "readall error: "+err.Error()+"\n")
			return
		}

		fmt.Printf(">>>%s\n", b)

		URL, err := url.Parse("https://api.jvl-cluster.develop.tarmak.org" + r.URL.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse error: "+err.Error()+"\n")
			return
		}

		req := &http.Request{
			URL: URL,
		}
		req.Header = copyHeader(r.Header)
		fmt.Printf(">%s\n", req.Header)
		fmt.Printf(">%s\n", r.Header)
		fmt.Printf("%+v\n", r.TLS)

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

func copyHeader(src http.Header) http.Header {
	dst := http.Header{}
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}

	dst.Del("Authorization")
	return dst
}

func setHeader(w http.ResponseWriter, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
}
