// Copyright Jetstack Ltd. See LICENSE for details.
package probe

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"k8s.io/apiserver/pkg/authentication/authenticator"

	"github.com/jetstack/kube-oidc-proxy/cmd/options"
	"github.com/jetstack/kube-oidc-proxy/pkg/util"
)

var _ authenticator.Token = &fakeAuthenticator{}

type fakeAuthenticator struct {
	readyBy time.Time
}

func (f *fakeAuthenticator) AuthenticateToken(context.Context, string) (*authenticator.Response, bool, error) {
	if time.Now().After(f.readyBy) {
		return nil, true, nil
	}

	return nil, false, errors.New("auther not ready")
}

func TestRun(t *testing.T) {
	tests := map[string]struct {
		becomeReady  bool
		path, method string
	}{
		"if becomes ready but bad path then always error": {
			becomeReady: true,
			path:        "/bad/path",
			method:      "GET",
		},
		"if becomes ready but bad method then always error": {
			becomeReady: true,
			path:        "/health",
			method:      "POST",
		},
		"if the authenticator never becomes ready then endpoint should never ready": {
			becomeReady: false,
			path:        "/health",
			method:      "GET",
		},
		"if the authenticator becomes ready then endpoint should become ready": {
			becomeReady: true,
			path:        "/health",
			method:      "GET",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			port, err := util.FreePort()
			if err != nil {
				t.Error(err)
				t.FailNow()
			}

			readyBy := time.Now()
			if !test.becomeReady {
				readyBy = time.Now().Add(time.Hour)
			}

			f := &fakeAuthenticator{readyBy}

			err = Run(port, f, new(options.OIDCAuthenticationOptions))
			if err != nil {
				t.Error(err)
				t.FailNow()
			}

			time.Sleep(time.Millisecond * 600)

			r, err := http.NewRequest(test.method,
				fmt.Sprintf("http://127.0.0.1:%s%s", port, test.path), nil)
			if err != nil {
				t.Error(err)
				t.FailNow()
			}

			resp, err := http.DefaultClient.Do(r)
			if err != nil {
				t.Error(err)
				t.FailNow()
			}

			// if wrong method then error
			if test.method != "GET" {
				if resp.StatusCode != http.StatusMethodNotAllowed {
					t.Errorf("got unexpected status code, exp=%d got=%d",
						http.StatusMethodNotAllowed, resp.StatusCode)
				}
				return
			}

			// if wrong path then error
			if test.path != "/health" {
				if resp.StatusCode != http.StatusNotFound {
					t.Errorf("got unexpected status code, exp=%d got=%d",
						http.StatusOK, resp.StatusCode)
				}
				return
			}

			// return status OK only if expected to be ready
			if test.becomeReady {
				if resp.StatusCode != http.StatusOK {
					t.Errorf("got unexpected status code, exp=%d got=%d",
						http.StatusOK, resp.StatusCode)
				}
			} else {
				if resp.StatusCode != http.StatusServiceUnavailable {
					t.Errorf("got unexpected status code, exp=%d got=%d",
						http.StatusServiceUnavailable, resp.StatusCode)
				}
			}
		})
	}
}
