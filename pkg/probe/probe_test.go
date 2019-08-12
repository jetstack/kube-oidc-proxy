// Copyright Jetstack Ltd. See LICENSE for details.
package probe

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jetstack/kube-oidc-proxy/pkg/util"
)

func Test_Check(t *testing.T) {
	port, err := util.FreePort()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	p := New(port)
	time.Sleep(time.Second)

	url := fmt.Sprintf("http://0.0.0.0:%s", port)

	resp, err := http.Get(url + "/ready")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if resp.StatusCode != 503 {
		t.Errorf("expected ready probe to be responding and not ready, exp=%d got=%d",
			503, resp.StatusCode)
	}

	p.SetReady()

	resp, err = http.Get(url + "/ready")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected ready probe to be responding and ready, exp=%d got=%d",
			200, resp.StatusCode)
	}

	p.SetNotReady()

	resp, err = http.Get(url + "/ready")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if resp.StatusCode != 503 {
		t.Errorf("expected ready probe to be responding and not ready, exp=%d got=%d",
			503, resp.StatusCode)
	}
}
