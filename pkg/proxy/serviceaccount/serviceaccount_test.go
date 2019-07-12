// Copyright Jetstack Ltd. See LICENSE for details.
package serviceaccount

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"k8s.io/client-go/rest"
	apiserveroptions "k8s.io/kubernetes/pkg/kubeapiserver/options"
)

func TestNew(t *testing.T) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "kube-oidc-proxy-sa")
	if err != nil {
		t.Errorf("failed to create tmp dir: %s", err)
		t.FailNow()
	}
	defer os.RemoveAll(tmpDir)

	var badKey, goodKey *os.File
	badKey, err = ioutil.TempFile(tmpDir, "bad-key.pub")
	if err != nil {
		t.Errorf("failed to create tmp key file: %s", err)
		t.FailNow()
	}

	goodKey, err = ioutil.TempFile(tmpDir, "good-key.pub")
	if err != nil {
		t.Errorf("failed to create tmp key file: %s", err)
		t.FailNow()
	}

	sk, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	//b, err := asn1.Marshal(sk.PublicKey)
	//if err != nil {
	//	t.Error(err)
	//	t.FailNow()
	//}

	publickeyencoder := gob.NewEncoder(goodKey)
	publickeyencoder.Encode(sk.PublicKey)

	//pkPEM := &pem.Block{
	//	Type:  "RSA PUBLIC KEY",
	//	Bytes: b,
	//}

	//_, err = goodKey.Write(pkPEM.Bytes)
	//if err != nil {
	//	t.Errorf("failed to write to bad key file: %s", err)
	//	t.FailNow()
	//}

	_, err = badKey.Write([]byte("bad key"))
	if err != nil {
		t.Errorf("failed to write to bad key file: %s", err)
		t.FailNow()
	}

	tests := map[string]struct {
		options         *apiserveroptions.ServiceAccountAuthenticationOptions
		err             error
		scopedAutherNil bool
	}{
		"if lookup is disabled, scoped auther should be nil": {
			options: &apiserveroptions.ServiceAccountAuthenticationOptions{
				Issuer:        "my-iss",
				KeyFiles:      nil,
				Lookup:        false,
				MaxExpiration: time.Minute,
			},
			err:             nil,
			scopedAutherNil: true,
		},
		"if lookup is enabled, scoped auther should be not nil": {
			options: &apiserveroptions.ServiceAccountAuthenticationOptions{
				Issuer:        "my-iss",
				KeyFiles:      nil,
				Lookup:        true,
				MaxExpiration: time.Minute,
			},
			err:             nil,
			scopedAutherNil: false,
		},
		"if lookup is enabled, scoped auther should be not nil but should fail with bad public key files": {
			options: &apiserveroptions.ServiceAccountAuthenticationOptions{
				Issuer:        "my-iss",
				KeyFiles:      []string{badKey.Name()},
				Lookup:        true,
				MaxExpiration: time.Minute,
			},
			err: fmt.Errorf(
				"error reading public key file %s: data does not contain any valid RSA or ECDSA public keys",
				badKey.Name(),
			),
			scopedAutherNil: false,
		},
		"if lookup is enabled, scoped auther should be not nil and should not fail with good public key files": {
			options: &apiserveroptions.ServiceAccountAuthenticationOptions{
				Issuer:        "my-iss",
				KeyFiles:      []string{goodKey.Name()},
				Lookup:        true,
				MaxExpiration: time.Minute,
			},
			err:             nil,
			scopedAutherNil: false,
		},
	}

	for n, c := range tests {
		auther, err := New(new(rest.Config), c.options, []string{"api"})

		if c.err != nil {
			if err == nil || err.Error() != c.err.Error() {
				t.Errorf("%s: unexpected error, exp=%s got=%v",
					n, c.err, err)
			}
			continue
		}

		if c.err == nil && err != nil {
			t.Errorf("%s: unexpected error, exp=%v got=%s",
				n, c.err, err)
			continue
		}

		if auther.legacyAuther == nil {
			t.Errorf("%s: unexpected legacy auther to be nil=false but got=%+v",
				n, auther.legacyAuther)
		}

		if (auther.scopedAuther == nil) != c.scopedAutherNil {
			t.Errorf("%s: unexpected scoped auther to be nil=%t but got=%+v",
				n, c.scopedAutherNil, auther.scopedAuther)
		}
	}
}

func TestParseTokenFromHeader(t *testing.T) {
	tests := map[string]struct {
		req   *http.Request
		token string
		err   error
	}{
		"should return error if request is nil": {
			req:   nil,
			token: "",
			err:   ErrTokenParse,
		},
		"should return error if request.Header is nil": {
			req: &http.Request{
				Header: nil,
			},
			token: "",
			err:   ErrTokenParse,
		},
		"should return error if no Authorization header given": {
			req: &http.Request{
				Header: map[string][]string{
					"Random-Header1": []string{"foo bar"},
					"Random-Header2": []string{"boo koo"},
				},
			},
			token: "",
			err:   ErrTokenParse,
		},
		"should return error if Authorization header is empty": {
			req: &http.Request{
				Header: map[string][]string{
					"Random-Header1": []string{"foo bar"},
					"Authorization":  []string{},
				},
			},
			token: "",
			err:   ErrTokenParse,
		},
		"should return error if Authorization header is only 'bearer'": {
			req: &http.Request{
				Header: map[string][]string{
					"Random-Header1": []string{"foo bar"},
					"Authorization":  []string{"bearer"},
				},
			},
			token: "",
			err:   ErrTokenParse,
		},
		"should return error if Authorization header is only 'bearertoken'": {
			req: &http.Request{
				Header: map[string][]string{
					"Random-Header1": []string{"foo bar"},
					"Authorization":  []string{"bearertoken"},
				},
			},
			token: "",
			err:   ErrTokenParse,
		},
		"should return 'token' if Authorization header is 'bearer token'": {
			req: &http.Request{
				Header: map[string][]string{
					"Random-Header1": []string{"foo bar"},
					"Authorization":  []string{"bearer token"},
				},
			},
			token: "token",
			err:   nil,
		},
		"should return error if Authorization header is 'bearer token' but not first element": {
			req: &http.Request{
				Header: map[string][]string{
					"Random-Header1": []string{"foo bar"},
					"Authorization":  []string{"foo bar", "bearer token"},
				},
			},
			token: "",
			err:   ErrTokenParse,
		},
		"should return 'token' if Authorization header is 'bearer token some-other-string'": {
			req: &http.Request{
				Header: map[string][]string{
					"Random-Header1": []string{"foo bar"},
					"Authorization":  []string{"bearer token some-other-string"},
				},
			},
			token: "token",
			err:   nil,
		},
		"should return 'token' if Authorization header is 'bearer token' but mixed capitals on bearer": {
			req: &http.Request{
				Header: map[string][]string{
					"Random-Header1": []string{"foo bar"},
					"Authorization":  []string{"BeAREr token"},
				},
			},
			token: "token",
			err:   nil,
		},
		"should return error if Authorization header is 'bearer token' but the header name is title capitalised": {
			req: &http.Request{
				Header: map[string][]string{
					"Random-Header1": []string{"foo bar"},
					"authorization":  []string{"bearer token"},
				},
			},
			token: "",
			err:   ErrTokenParse,
		},
		"should return error if Authorization header has multiple spaces between 'bearer' and 'token'": {
			req: &http.Request{
				Header: map[string][]string{
					"Random-Header1": []string{"foo bar"},
					"Authorization":  []string{"bearer     token"},
				},
			},
			token: "",
			err:   ErrTokenParse,
		},
	}

	for n, c := range tests {
		gToken, gErr := parseTokenFromHeader(c.req)

		if c.err != gErr {
			t.Errorf("%s: unexpected error, exp=%s got=%s",
				n, c.err, gErr)
		}

		if c.token != gToken {
			t.Errorf("%s: unexpected token, exp=%s got=%s",
				n, c.token, gToken)
		}
	}
}
