// Copyright Jetstack Ltd. See LICENSE for details.
package probe

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"k8s.io/apiserver/pkg/authentication/authenticator"

	"github.com/jetstack/kube-oidc-proxy/pkg/util"
)

type fakeTokenAuthenticator struct {
	returnErr bool
}

var _ authenticator.Token = &fakeTokenAuthenticator{}

func (f *fakeTokenAuthenticator) AuthenticateToken(context.Context, string) (*authenticator.Response, bool, error) {
	if f.returnErr {
		return nil, false, errors.New("foo bar authenticator not initialized")
	}

	return nil, false, errors.New("some other error")
}

func TestRun(t *testing.T) {
	f := &fakeTokenAuthenticator{
		returnErr: true,
	}

	port, err := util.FreePort()
	if err != nil {
		t.Error(err.Error())
		t.FailNow()
	}

	fakeJWT, err := util.FakeJWT("issuer")
	if err != nil {
		t.Error(err.Error())
		t.FailNow()
	}

	readinessHandler, err := Run(port, fakeJWT, f)
	if err != nil {
		t.Error(err.Error())
		t.FailNow()
	}

	defer func() {
		if err := readinessHandler.Shutdown(); err != nil {
			t.Error(err)
		}
	}()

	url := fmt.Sprintf("http://0.0.0.0:%s", port)

	var resp *http.Response
	var i int

	for {
		resp, err = http.Get(url + "/ready")
		if err == nil {
			break
		}

		if i >= 5 {
			t.Errorf("unexpected error: %s", err)
			t.FailNow()
		}
		i++
	}

	if resp.StatusCode != 503 {
		t.Errorf("expected ready probe to be responding and not ready, exp=%d got=%d",
			503, resp.StatusCode)
	}

	f.returnErr = false

	resp, err = http.Get(url + "/ready")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected ready probe to be responding and ready, exp=%d got=%d",
			200, resp.StatusCode)
	}

	// Once the authenticator has returned with an non-initialised error, then
	// should always return ready

	f.returnErr = true

	resp, err = http.Get(url + "/ready")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected ready probe to be responding and ready, exp=%d got=%d",
			200, resp.StatusCode)
	}
}
