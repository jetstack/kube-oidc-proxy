// Copyright Jetstack Ltd. See LICENSE for details.
package proxy

import (
	"bytes"
	"errors"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/apiserver/pkg/authentication/user"
	authuser "k8s.io/apiserver/pkg/authentication/user"

	"github.com/jetstack/kube-oidc-proxy/pkg/mocks"
)

type fakeProxy struct {
	ctrl *gomock.Controller

	fakeToken *mocks.MockToken
	fakeRT    *fakeRT
	*Proxy
}

type fakeRW struct {
	buffer []byte
	header http.Header
}

type fakeRT struct {
	t *testing.T

	expUser  string
	expGroup []string
	expExtra map[string][]string
}

func (f *fakeRW) Write(b []byte) (int, error) {
	f.buffer = append(f.buffer, b...)
	return len(b), nil
}

func (f *fakeRW) WriteHeader(code int) {
	f.header.Add("StatusCode", strconv.Itoa(code))
}

func (f *fakeRW) Header() http.Header {
	return f.header
}

func newFakeR() *http.Request {
	return &http.Request{
		RemoteAddr: "fakeAddr",
	}
}

func newFakeRW() *fakeRW {
	return &fakeRW{
		header: make(http.Header),
		buffer: make([]byte, 0),
	}
}

func (f *fakeRT) RoundTrip(h *http.Request) (*http.Response, error) {
	if h.Header.Get("Impersonate-User") != f.expUser {
		f.t.Errorf("client transport got unexpected user impersonation header, exp=%s got=%s",
			f.expUser, h.Header.Get("Impersonate-User"))
	}

	if exp, act := sort.StringSlice(f.expGroup), sort.StringSlice(h.Header["Impersonate-Group"]); !reflect.DeepEqual(exp, act) {
		f.t.Errorf(
			"client transport got unexpected group impersonation header, exp=%#v got=%#v",
			exp,
			act,
		)
	}

	for k, vv := range h.Header {
		if strings.HasPrefix(k, "Impersonate-Extra-") {
			expvv, ok := f.expExtra[k]
			if !ok {
				f.t.Errorf("got unexpected impersonate extra: %s", k)
				continue
			}

			if !reflect.DeepEqual(vv, expvv) {
				f.t.Errorf("unexpected values in impersonate extra (%s), exp=%s got=%s", k, expvv, vv)
			}
		}
	}

	for k, expvv := range f.expExtra {
		vv, ok := h.Header[k]
		if !ok {
			f.t.Errorf("did not get expected impersonate extra: %s", k)
			continue
		}

		if !reflect.DeepEqual(vv, expvv) {
			f.t.Errorf("unexpected values in impersonate extra (%s), exp=%s got=%s", k, expvv, vv)
		}
	}

	return nil, nil
}

func tryError(t *testing.T, expCode int, err error) *fakeRW {
	p := new(Proxy)

	frw := newFakeRW()
	fr := newFakeR()

	p.Error(frw, fr, err)

	code, err := strconv.Atoi(frw.header.Get("StatusCode"))
	if err != nil {
		t.Errorf(
			"failed to get status code from response header: %s",
			err)
	}

	if code != expCode {
		t.Errorf("unexpected status code, exp=%d got=%d",
			expCode, code)
	}

	return frw
}

func Test_Error(t *testing.T) {
	// no error
	frw := tryError(t, http.StatusInternalServerError, nil)
	if len(frw.buffer) != 1 {
		t.Errorf("unexpected response, exp='\n' got='%s'", frw.buffer)
	}

	frw = tryError(t, http.StatusUnauthorized, errUnauthorized)
	if exp := []byte("Unauthorized\n"); !bytes.Equal(frw.buffer, exp) {
		t.Errorf("unexpected response, exp='%s' got='%s'", exp, frw.buffer)
	}

	frw = tryError(t, http.StatusForbidden, errImpersonateHeader)
	if exp := []byte("Impersonation requests are disabled when using kube-oidc-proxy\n"); !bytes.Equal(frw.buffer, exp) {
		t.Errorf("unexpected response, exp='%s' got='%s'", exp, frw.buffer)
	}

	frw = tryError(t, http.StatusForbidden, errNoName)
	if exp := []byte("Username claim not available in OIDC Issuer response\n"); !bytes.Equal(frw.buffer, exp) {
		t.Errorf("unexpected response, exp='%s' got='%s'", exp, frw.buffer)
	}

	frw = tryError(t, http.StatusInternalServerError, errors.New("foo"))
	if exp := []byte("\n"); !bytes.Equal(frw.buffer, exp) {
		t.Errorf("unexpected response, exp='%s' got='%s'", exp, frw.buffer)
	}
}

func Test_hasImpersonation(t *testing.T) {
	p := new(Proxy)

	// no impersonation headers
	noImpersonation := []http.Header{
		{},
		{
			"foo": []string{"bar", "foo"},
		},
		{
			"impersonation": []string{"bar"},
			"impersonate":   []string{"bar"},
		},
		{
			"Impersonate": []string{"bar", "foo"},
		},
		{
			"impersonate-Extra": []string{"bar", "foo"},
		},
		{
			"-impersonate-Extra-": []string{"bar", "foo"},
		},
		{
			"a": []string{"Impersonate-User"},
			"b": []string{"Impersonate-Group"},
			"c": []string{"Impersonate-Extra-"},
		},
	}

	// impersonation headers
	hasImpersonation := []http.Header{
		{
			"Impersonate-User": []string{"bar", "foo"},
		},
		{
			"impersonate-user": []string{"bar", "foo"},
		},
		{
			"impersonate-user": nil,
		},
		{
			"impersonate-group": nil,
		},
		{
			"impersonate-Group": []string{"bar", "foo"},
		},
		{
			"impersonate-Extra-foobar___foo": []string{"bar", "foo"},
		},
		{
			"impersonate-Extra-": []string{"bar", "foo"},
		},
		{
			"impersonate-Extra-": []string{"bar", "foo"},
			"impersonate-Group":  []string{"bar", "foo"},
			"impersonate-User":   []string{"bar"},
		},
		{
			"impersonate-Extra-": []string{"bar", "foo"},
			"foo":                []string{"bar", "foo"},
			"bar":                []string{"bar"},
		},
		{
			"foo":                []string{"bar", "foo"},
			"impersonate-Extra-": []string{"bar", "foo"},
			"bar":                []string{"bar"},
			"impersonate-Group":  []string{"bar", "foo"},
			"foo2":               []string{"bar", "foo"},
			"impersonate-User":   []string{"bar"},
			"bar2":               []string{"bar"},
		},
	}

	for _, h := range noImpersonation {
		if p.hasImpersonation(h) {
			t.Errorf("expected no impersonation but got true, '%s'", h)
		}
	}

	for _, h := range hasImpersonation {
		if !p.hasImpersonation(h) {
			t.Errorf("expected impersonation but got false, '%s'", h)
		}
	}
}

func newTestProxy(t *testing.T) *fakeProxy {
	ctrl := gomock.NewController(t)
	fakeToken := mocks.NewMockToken(ctrl)
	fakeRT := &fakeRT{t: t}

	return &fakeProxy{
		ctrl:      ctrl,
		fakeToken: fakeToken,
		fakeRT:    fakeRT,
		Proxy: &Proxy{
			reqAuther:       bearertoken.New(fakeToken),
			clientTransport: fakeRT,
		},
	}
}

func Test_RoundTrip(t *testing.T) {
	p := newTestProxy(t)
	_, err := p.RoundTrip(new(http.Request))
	if err != errUnauthorized {
		t.Errorf("unexpected round trip error, exp=%s got=%s", errUnauthorized, err)
	}

	req := &http.Request{
		Header: http.Header{
			"Authorization": []string{"foo"},
		},
	}
	_, err = p.RoundTrip(req)
	if err != errUnauthorized {
		t.Errorf("unexpected round trip error, exp=%s got=%s", errUnauthorized, err)
	}

	p.fakeToken.EXPECT().AuthenticateToken(gomock.Any(), "fake-token").Return(nil, false, nil)
	req.Header = http.Header{
		"Authorization": []string{"bearer fake-token"},
	}
	_, err = p.RoundTrip(req)
	if err != errUnauthorized {
		t.Errorf("unexpected round trip error, exp=%s got=%s", errUnauthorized, err)
	}

	p.fakeToken.EXPECT().AuthenticateToken(gomock.Any(), "fake-token").Return(nil, false, errors.New("some error"))
	req.Header = http.Header{
		"Authorization": []string{"bearer fake-token"},
	}
	_, err = p.RoundTrip(req)
	if err != errUnauthorized {
		t.Errorf("unexpected round trip error, exp=%s got=%s", errUnauthorized, err)
	}

	p.fakeToken.EXPECT().AuthenticateToken(gomock.Any(), "fake-token").Return(nil, true, errors.New("some error"))
	req.Header = http.Header{
		"Authorization": []string{"bearer fake-token"},
	}
	_, err = p.RoundTrip(req)
	if err != errUnauthorized {
		t.Errorf("unexpected round trip error, exp=%s got=%s", errUnauthorized, err)
	}

	// we fail at unauthorized even though we have impersonation headers
	p.fakeToken.EXPECT().AuthenticateToken(gomock.Any(), "fake-token").Return(nil, true, errors.New("some error"))
	req.Header = http.Header{
		"Authorization":    []string{"bearer fake-token"},
		"Impersonate-User": []string{"a-user"},
	}
	_, err = p.RoundTrip(req)
	if err != errUnauthorized {
		t.Errorf("unexpected round trip error, exp=%s got=%s", errUnauthorized, err)
	}

	authResponse := &authenticator.Response{
		User: &user.DefaultInfo{},
	}
	p.fakeToken.EXPECT().AuthenticateToken(gomock.Any(), "fake-token").Return(authResponse, true, nil)
	req.Header = http.Header{
		"Authorization":    []string{"bearer fake-token"},
		"Impersonate-User": []string{"a-user"},
	}
	_, err = p.RoundTrip(req)
	if err != errImpersonateHeader {
		t.Errorf("unexpected round trip error, exp=%s got=%s", errImpersonateHeader, err)
	}

	p.fakeToken.EXPECT().AuthenticateToken(gomock.Any(), "fake-token").Return(authResponse, true, nil)
	req.Header = http.Header{
		"Authorization":     []string{"bearer fake-token"},
		"Impersonate-Group": []string{"a-user"},
	}
	_, err = p.RoundTrip(req)
	if err != errImpersonateHeader {
		t.Errorf("unexpected round trip error, exp=%s got=%s", errImpersonateHeader, err)
	}

	p.fakeToken.EXPECT().AuthenticateToken(gomock.Any(), "fake-token").Return(authResponse, true, nil)
	req.Header = http.Header{
		"Authorization":         []string{"bearer fake-token"},
		"Impersonate-Extra-foo": []string{"a-user"},
	}
	_, err = p.RoundTrip(req)
	if err != errImpersonateHeader {
		t.Errorf("unexpected round trip error, exp=%s got=%s", errImpersonateHeader, err)
	}

	p.fakeToken.EXPECT().AuthenticateToken(gomock.Any(), "fake-token").Return(authResponse, true, nil)
	req.Header = http.Header{
		"Authorization": []string{"bearer fake-token"},
	}
	_, err = p.RoundTrip(req)
	if err != errNoName {
		t.Errorf("unexpected round trip error, exp=%s got=%s", errNoName, err)
	}

	authResponse = &authenticator.Response{
		User: &user.DefaultInfo{
			Name: "a-user",
		},
	}
	p.fakeToken.EXPECT().AuthenticateToken(gomock.Any(), "fake-token").Return(authResponse, true, nil)
	p.fakeRT.expUser = "a-user"
	p.fakeRT.expGroup = []string{authuser.AllAuthenticated}
	req.Header["Authorization"] = []string{"bearer fake-token"}
	_, err = p.RoundTrip(req)
	if err != nil {
		t.Errorf("unexpected round trip error, exp=nil got=%s", err)
	}

	authResponse = &authenticator.Response{
		User: &user.DefaultInfo{
			Name:   "a-user",
			Groups: []string{"a-group-a", "a-group-b", authuser.AllAuthenticated},
			Extra: map[string][]string{
				"foo":     []string{"a", "b"},
				"bar":     []string{"c", "d"},
				"foo-bar": []string{"e", "f"},
			},
		},
	}
	p.fakeToken.EXPECT().AuthenticateToken(gomock.Any(), "fake-token").Return(authResponse, true, nil)
	p.fakeRT.expUser = "a-user"
	p.fakeRT.expGroup = []string{"a-group-a", "a-group-b", authuser.AllAuthenticated}
	p.fakeRT.expExtra = map[string][]string{
		"Impersonate-Extra-Foo":     []string{"a", "b"},
		"Impersonate-Extra-Bar":     []string{"c", "d"},
		"Impersonate-Extra-Foo-Bar": []string{"e", "f"},
	}
	req.Header["Authorization"] = []string{"bearer fake-token"}
	_, err = p.RoundTrip(req)
	if err != nil {
		t.Errorf("unexpected round trip error, exp=nil got=%s", err)
	}
}
