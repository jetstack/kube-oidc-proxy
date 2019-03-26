// Copyright Jetstack Ltd. See LICENSE for details.
package probe

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/jetstack/kube-oidc-proxy/pkg/utils"
)

func Test_Check(t *testing.T) {
	port, err := utils.FreePort()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	p, err := New(port)
	if err != nil {
		t.Fatal(err)
	}

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

func Test_New(t *testing.T) {
	port, err := utils.FreePort()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	ln, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", port))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := New(port); err == nil {
		t.Errorf("expected error port taken, got=%s", err)
	} else {
		t.Logf("got expected port taken error: %s", err)
	}

	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := New(port); err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}
