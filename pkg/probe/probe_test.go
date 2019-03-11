// Copyright Jetstack Ltd. See LICENSE for details.
package probe

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func Test_Check(t *testing.T) {
	port, err := freePort()
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

func freePort() (string, error) {
	l, err := net.ListenTCP("tcp", &net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0,
	})
	if err != nil {
		return "", err
	}
	defer l.Close()

	port := l.Addr().(*net.TCPAddr).Port
	return strconv.Itoa(port), nil
}
