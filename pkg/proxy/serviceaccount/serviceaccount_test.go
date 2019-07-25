// Copyright Jetstack Ltd. See LICENSE for details.
package serviceaccount

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"k8s.io/client-go/rest"

	"github.com/jetstack/kube-oidc-proxy/cmd/options"
	"github.com/jetstack/kube-oidc-proxy/pkg/mocks"
)

func TestNew(t *testing.T) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "kube-oidc-proxy-sa")
	if err != nil {
		t.Errorf("failed to create tmp dir: %s", err)
		t.FailNow()
	}
	defer os.RemoveAll(tmpDir)

	var badKey, goodKey *os.File
	badKey, err = os.Create(filepath.Join(tmpDir, "bad-key.pub"))
	if err != nil {
		t.Errorf("failed to create tmp key file: %s", err)
		t.FailNow()
	}

	goodKey, err = os.Create(filepath.Join(tmpDir, "good-key.pub"))
	if err != nil {
		t.Errorf("failed to create tmp key file: %s", err)
		t.FailNow()
	}

	sk, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	der, err := x509.MarshalPKIXPublicKey(sk.Public())
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	block := pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: der,
	}
	pem.Encode(goodKey, &block)

	_, err = badKey.Write([]byte("bad key"))
	if err != nil {
		t.Errorf("failed to write to bad key file: %s", err)
		t.FailNow()
	}

	tests := map[string]struct {
		options         *options.ServiceAccountAuthenticationOptions
		err             error
		scopedAutherNil bool
	}{
		"if lookup is disabled, scoped auther should be nil": {
			options: &options.ServiceAccountAuthenticationOptions{
				Issuer:        "my-iss",
				KeyFiles:      nil,
				Lookup:        false,
				MaxExpiration: time.Minute,
			},
			err:             nil,
			scopedAutherNil: true,
		},
		"if lookup is enabled, scoped auther should be not nil": {
			options: &options.ServiceAccountAuthenticationOptions{
				Issuer:        "my-iss",
				KeyFiles:      nil,
				Lookup:        true,
				MaxExpiration: time.Minute,
			},
			err:             nil,
			scopedAutherNil: false,
		},
		"if lookup is enabled, scoped auther should be not nil but should fail with bad public key files": {
			options: &options.ServiceAccountAuthenticationOptions{
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
			options: &options.ServiceAccountAuthenticationOptions{
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

func TestRequest(t *testing.T) {
	req := &http.Request{
		Header: map[string][]string{
			"Authorization": []string{"bearer my-token"},
		},
	}

	tests := map[string]struct {
		req    *http.Request
		lookup bool
		expect func(legacy, scoped *mocks.MockToken)
		err    error
	}{
		"if token fails to parse, should return error and not call auth": {
			req: &http.Request{
				Header: map[string][]string{
					"Authorization": []string{"bad-token"},
				},
			},
			lookup: false,
			expect: func(legacy, scoped *mocks.MockToken) {
				legacy.EXPECT().AuthenticateToken(gomock.Any(), gomock.Any()).Times(0)
				scoped.EXPECT().AuthenticateToken(gomock.Any(), gomock.Any()).Times(0)
			},
			err: ErrTokenParse,
		},
		"if lookup disabled and legacy token errors, scoped not called and errors": {
			req:    req,
			lookup: false,
			expect: func(legacy, scoped *mocks.MockToken) {
				scoped.EXPECT().AuthenticateToken(gomock.Any(), gomock.Any()).Times(0)
				legacy.EXPECT().AuthenticateToken(gomock.Any(), "my-token").Times(1).
					Return(nil, false, errors.New(""))
			},
			err: ErrUnAuthed,
		},
		"if lookup disabled and legacy token fails, scoped not called and errors": {
			req:    req,
			lookup: false,
			expect: func(legacy, scoped *mocks.MockToken) {
				scoped.EXPECT().AuthenticateToken(gomock.Any(), gomock.Any()).Times(0)
				legacy.EXPECT().AuthenticateToken(gomock.Any(), "my-token").Times(1).
					Return(nil, false, nil)
			},
			err: ErrUnAuthed,
		},
		"if lookup disabled and legacy token passes, return nil": {
			req:    req,
			lookup: false,
			expect: func(legacy, scoped *mocks.MockToken) {
				scoped.EXPECT().AuthenticateToken(gomock.Any(), gomock.Any()).Times(0)
				legacy.EXPECT().AuthenticateToken(gomock.Any(), "my-token").Times(1).
					Return(nil, true, nil)
			},
			err: nil,
		},
		"if lookup enabled and both legacy & scoped token errors, return error": {
			req:    req,
			lookup: true,
			expect: func(legacy, scoped *mocks.MockToken) {
				scoped.EXPECT().AuthenticateToken(gomock.Any(), "my-token").Times(1).
					Return(nil, false, errors.New(""))
				legacy.EXPECT().AuthenticateToken(gomock.Any(), "my-token").Times(1).
					Return(nil, false, errors.New(""))
			},
			err: ErrUnAuthed,
		},
		"if lookup enabled and both legacy & scoped token fails, return error": {
			req:    req,
			lookup: true,
			expect: func(legacy, scoped *mocks.MockToken) {
				scoped.EXPECT().AuthenticateToken(gomock.Any(), "my-token").Times(1).
					Return(nil, false, nil)
				legacy.EXPECT().AuthenticateToken(gomock.Any(), "my-token").Times(1).
					Return(nil, false, nil)
			},
			err: ErrUnAuthed,
		},
		"if lookup enabled and scoped token fails but legacy passes, return nil": {
			req:    req,
			lookup: true,
			expect: func(legacy, scoped *mocks.MockToken) {
				scoped.EXPECT().AuthenticateToken(gomock.Any(), "my-token").Times(1).
					Return(nil, false, nil)
				legacy.EXPECT().AuthenticateToken(gomock.Any(), "my-token").Times(1).
					Return(nil, true, nil)
			},
			err: nil,
		},
		"if lookup enabled and scoped token passes, legacy shouldn't be called and return nil": {
			req:    req,
			lookup: true,
			expect: func(legacy, scoped *mocks.MockToken) {
				scoped.EXPECT().AuthenticateToken(gomock.Any(), "my-token").Times(1).
					Return(nil, true, nil)
				legacy.EXPECT().AuthenticateToken(gomock.Any(), "my-token").Times(0)
			},
			err: nil,
		},
	}

	for n, c := range tests {
		ctrl := gomock.NewController(t)
		legacyAuther := mocks.NewMockToken(ctrl)
		scopedAuther := mocks.NewMockToken(ctrl)

		c.expect(legacyAuther, scopedAuther)

		a := &Authenticator{
			legacyAuther: legacyAuther,
			scopedAuther: scopedAuther,
			lookup:       c.lookup,
		}

		err := a.Request(c.req)
		if err != c.err {
			t.Errorf("%s: unexpected error, exp=%s got=%s",
				n, c.err, err)
		}
		ctrl.Finish()
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
