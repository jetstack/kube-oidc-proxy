// Copyright Jetstack Ltd. See LICENSE for details.
package proxy

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/server"

	"github.com/jetstack/kube-oidc-proxy/cmd/app/options"
	"github.com/jetstack/kube-oidc-proxy/pkg/mocks"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/audit"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/hooks"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/logging"
	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/subjectaccessreview"
	fakesubjectaccessreview "github.com/jetstack/kube-oidc-proxy/pkg/proxy/subjectaccessreview/fake"
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
	expUid   string
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
		logging.LogFailedRequest(h)
		f.t.Errorf("client transport got unexpected user impersonation header, exp=%s got=%s",
			f.expUser, h.Header.Get("Impersonate-User"))
	}

	if h.Header.Get("Impersonate-Uid") != f.expUid {
		logging.LogFailedRequest(h)
		f.t.Errorf("client transport got unexpected uid impersonation header, exp=%s got=%s",
			f.expUid, h.Header.Get("Impersonate-Uid"))
	}

	if exp, act := sort.StringSlice(f.expGroup), sort.StringSlice(h.Header["Impersonate-Group"]); !reflect.DeepEqual(exp, act) {
		logging.LogFailedRequest(h)
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
				logging.LogFailedRequest(h)
				f.t.Errorf("got unexpected impersonate extra: %s", k)
				continue
			}

			if !reflect.DeepEqual(vv, expvv) {
				logging.LogFailedRequest(h)
				f.t.Errorf("unexpected values in impersonate extra (%s), exp=%s got=%s", k, expvv, vv)
			}
		}
	}

	for k, expvv := range f.expExtra {
		vv, ok := h.Header[k]
		if !ok {
			logging.LogFailedRequest(h)
			f.t.Errorf("did not get expected impersonate extra: %s", k)
			continue
		}

		if !reflect.DeepEqual(vv, expvv) {
			logging.LogFailedRequest(h)
			f.t.Errorf("unexpected values in impersonate extra (%s), exp=%s got=%s", k, expvv, vv)
		}
	}

	logging.LogSuccessfulRequest(h, &user.DefaultInfo{}, &user.DefaultInfo{})

	return nil, nil
}

func tryError(t *testing.T, expCode int, err error) *fakeRW {
	p := new(Proxy)
	p.handleError = p.newErrorHandler()

	frw := newFakeRW()
	fr := newFakeR()

	p.handleError(frw, fr, err)

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

func TestError(t *testing.T) {
	// no error
	frw := tryError(t, http.StatusInternalServerError, nil)
	if len(frw.buffer) != 1 {
		t.Errorf("unexpected response, exp='\n' got='%s'", frw.buffer)
	}

	frw = tryError(t, http.StatusUnauthorized, errUnauthorized)
	if exp := []byte("Unauthorized\n"); !bytes.Equal(frw.buffer, exp) {
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

func TestHasImpersonation(t *testing.T) {
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
		// any attempt to user impersonate- should be interpreted as
		// an impersonation header since it could be in the future
		{
			"impersonate-Extra": []string{"bar", "foo"},
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
	fakeSubjectAccessReviewer := fakesubjectaccessreview.New(nil)
	subjectAccessReview, _ := subjectaccessreview.New(fakeSubjectAccessReviewer)

	p := &fakeProxy{
		ctrl:      ctrl,
		fakeToken: fakeToken,
		fakeRT:    fakeRT,
		Proxy: &Proxy{
			oidcRequestAuther:     bearertoken.New(fakeToken),
			subjectAccessReviewer: subjectAccessReview,
			clientTransport:       fakeRT,
			noAuthClientTransport: fakeRT,
			config:                new(Config),
			hooks:                 hooks.New(),
		},
	}

	auditor, err := audit.New(new(options.AuditOptions), "0.0.0.0:1234", new(server.SecureServingInfo))
	if err != nil {
		t.Fatalf("failed to create auditor: %s", err)
	}
	p.auditor = auditor

	p.handleError = p.newErrorHandler()

	return p
}

func TestHandlers(t *testing.T) {
	type authResponse struct {
		resp *authenticator.Response
		pass bool
		err  error
	}

	tests := map[string]struct {
		req    *http.Request
		config *Config

		expAuthToken string
		authResponse *authResponse

		expCode int
		expBody string

		expUser  string
		expGroup []string
		expExtra map[string][]string
		expUid   string
	}{
		"an empty request should 401": {
			req:     new(http.Request),
			expCode: http.StatusUnauthorized,
			expBody: errUnauthorized.Error(),
		},
		"a request with a badly formed token should 401": {
			req: &http.Request{
				Header: http.Header{
					"Authorization": []string{"foo"},
				},
			},
			expCode: http.StatusUnauthorized,
			expBody: errUnauthorized.Error(),
		},
		"a request with a unauthed token should 401": {
			req: &http.Request{
				Header: http.Header{
					"Authorization": []string{"bearer fake-token"},
				},
			},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: nil,
				pass: false,
				err:  nil,
			},
			expCode: http.StatusUnauthorized,
			expBody: errUnauthorized.Error(),
		},
		"a request with an error during token auth should 401": {
			req: &http.Request{
				Header: http.Header{
					"Authorization": []string{"bearer fake-token"},
				},
			},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: nil,
				pass: false,
				err:  errors.New("some error"),
			},
			expCode: http.StatusUnauthorized,
			expBody: errUnauthorized.Error(),
		},
		"a request with an error but passes during token auth should still 401": {
			req: &http.Request{
				Header: http.Header{
					"Authorization": []string{"bearer fake-token"},
				},
			},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: nil,
				pass: true,
				err:  errors.New("some error"),
			},
			expCode: http.StatusUnauthorized,
			expBody: errUnauthorized.Error(),
		},
		"a request with unauth with impersonation should 401": {
			req: &http.Request{
				Header: http.Header{
					"Authorization":    []string{"bearer fake-token"},
					"Impersonate-User": []string{"a-user"},
				},
			},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: nil,
				pass: false,
				err:  nil,
			},
			expCode: http.StatusUnauthorized,
			expBody: errUnauthorized.Error(),
		},

		// BEGIN IMPERSONATION TESTS

		"an authed request with authorized impersonation user should succeed": {
			req: &http.Request{
				Header: http.Header{
					"Authorization":    []string{"bearer fake-token"},
					"Impersonate-User": []string{"jjackson"},
				},
			},
			expUser:      "jjackson",
			expGroup:     []string{"system:authenticated"},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: &authenticator.Response{
					User: &user.DefaultInfo{
						Name:   "mmosley",
						Groups: []string{"group1"},
					},
				},
				pass: true,
				err:  nil,
			},
			expCode: http.StatusOK,
			expExtra: map[string][]string{
				"Impersonate-Extra-Originaluser.jetstack.io-User":   {"mmosley"},
				"Impersonate-Extra-Originaluser.jetstack.io-Groups": {"group1"},
			},
			expBody: "",
		},
		"an authed request with authorized impersonation group should succeed": {
			req: &http.Request{
				Header: http.Header{
					"Authorization":     []string{"bearer fake-token"},
					"Impersonate-User":  []string{"jjackson"},
					"Impersonate-Group": []string{"group3"},
				},
			},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: &authenticator.Response{
					User: &user.DefaultInfo{
						Name:   "mmosley",
						Groups: []string{"group1"},
					},
				},
				pass: true,
				err:  nil,
			},
			expUser:  "jjackson",
			expGroup: []string{"group3", "system:authenticated"},
			expExtra: map[string][]string{
				"Impersonate-Extra-Originaluser.jetstack.io-User":   {"mmosley"},
				"Impersonate-Extra-Originaluser.jetstack.io-Groups": {"group1"},
			},
			expCode: http.StatusOK,
			expBody: "",
		},
		"an authed request with authorized impersonation extra should succeed": {
			req: &http.Request{
				Header: http.Header{
					"Authorization":                []string{"bearer fake-token"},
					"Impersonate-User":             []string{"jjackson"},
					"Impersonate-Group":            []string{"group3"},
					"Impersonate-Extra-remoteaddr": []string{"1.2.3.4"},
				},
			},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: &authenticator.Response{
					User: &user.DefaultInfo{
						Name:   "mmosley",
						Groups: []string{"group1"},
						Extra:  map[string][]string{"someextra": {"someval1", "someval2"}, "someextra2": {"foo", "bar"}},
					},
				},
				pass: true,
				err:  nil,
			},
			expCode:  http.StatusOK,
			expUser:  "jjackson",
			expGroup: []string{"group3", "system:authenticated"},
			expExtra: map[string][]string{
				"Impersonate-Extra-Remoteaddr":                      {"1.2.3.4"},
				"Impersonate-Extra-Originaluser.jetstack.io-User":   {"mmosley"},
				"Impersonate-Extra-Originaluser.jetstack.io-Groups": {"group1"},
				"Impersonate-Extra-Originaluser.jetstack.io-Extra":  {"{\"someextra\":[\"someval1\",\"someval2\"],\"someextra2\":[\"foo\",\"bar\"]}"},
			},
			expBody: "",
		},

		"an authed request with authorized impersonation extra should succeed, with an empty X-Forwarded-For header": {
			req: &http.Request{
				Header: http.Header{
					"Authorization":                []string{"bearer fake-token"},
					"Impersonate-User":             []string{"jjackson"},
					"Impersonate-Group":            []string{"group3"},
					"Impersonate-Extra-remoteaddr": []string{"1.2.3.4"},
					"X-Forwarded-For":              []string{""},
				},
			},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: &authenticator.Response{
					User: &user.DefaultInfo{
						Name:   "mmosley",
						Groups: []string{"group1"},
						Extra:  map[string][]string{"someextra": {"someval1", "someval2"}, "someextra2": {"foo", "bar"}},
					},
				},
				pass: true,
				err:  nil,
			},
			expCode:  http.StatusOK,
			expUser:  "jjackson",
			expGroup: []string{"group3", "system:authenticated"},
			expExtra: map[string][]string{
				"Impersonate-Extra-Remoteaddr":                      {"1.2.3.4"},
				"Impersonate-Extra-Originaluser.jetstack.io-User":   {"mmosley"},
				"Impersonate-Extra-Originaluser.jetstack.io-Groups": {"group1"},
				"Impersonate-Extra-Originaluser.jetstack.io-Extra":  {"{\"someextra\":[\"someval1\",\"someval2\"],\"someextra2\":[\"foo\",\"bar\"]}"},
			},
			expBody: "",
		},

		/* Commenting due to https://github.com/TremoloSecurity/kube-oidc-proxy/issues/7
		"an authed request with authorized impersonation uid should succeed": {
			req: &http.Request{
				Header: http.Header{
					"Authorization":     []string{"bearer fake-token"},
					"Impersonate-Uid":   []string{"1-2-3-4"},
					"Impersonate-User":  []string{"jjackson"},
					"Impersonate-Group": []string{"group3"},
				},
			},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: &authenticator.Response{
					User: &user.DefaultInfo{
						Name:   "mmosley",
						Groups: []string{"group1"},
					},
				},
				pass: true,
				err:  nil,
			},
			expCode:  http.StatusOK,
			expUser:  "jjackson",
			expUid:   "1-2-3-4",
			expGroup: []string{"group3", "system:authenticated"},
			expExtra: map[string][]string{
				"Impersonate-Extra-Originaluser.jetstack.io-User":   {"mmosley"},
				"Impersonate-Extra-Originaluser.jetstack.io-Groups": {"group1"},
			},
			expBody: "",
		},*/

		"an authed request with unauthorized impersonation user should error unauthorized": {
			req: &http.Request{
				Header: http.Header{
					"Authorization":    []string{"bearer fake-token"},
					"Impersonate-User": []string{"a-user"},
				},
			},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: &authenticator.Response{
					User: &user.DefaultInfo{
						Name:   "mmosley",
						Groups: []string{"group1"},
					},
				},
				pass: true,
				err:  nil,
			},
			expCode: http.StatusForbidden,
			expBody: "mmosley is not allowed to impersonate user 'a-user'",
		},
		"an authed request with unauthorized impersonation group should error unauthorized": {
			req: &http.Request{
				Header: http.Header{
					"Authorization":     []string{"bearer fake-token"},
					"Impersonate-User":  []string{"jjackson"},
					"Impersonate-Group": []string{"a-group"},
				},
			},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: &authenticator.Response{
					User: &user.DefaultInfo{
						Name:   "mmosley",
						Groups: []string{"group1"},
					},
				},
				pass: true,
				err:  nil,
			},
			expCode: http.StatusForbidden,
			expBody: "mmosley is not allowed to impersonate group 'a-group'",
		},
		"an authed request with unauthorized impersonation extra should error unauthorized": {
			req: &http.Request{
				Header: http.Header{
					"Authorization":         []string{"bearer fake-token"},
					"Impersonate-User":      []string{"jjackson"},
					"Impersonate-Extra-foo": []string{"bar"},
				},
			},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: &authenticator.Response{
					User: &user.DefaultInfo{
						Name:   "mmosley",
						Groups: []string{"group1"},
					},
				},
				pass: true,
				err:  nil,
			},
			expCode: http.StatusForbidden,
			expBody: "mmosley is not allowed to impersonate extra info 'foo'='bar'",
		},
		"an authed request with unauthorized impersonation uid should error unauthorized": {
			req: &http.Request{
				Header: http.Header{
					"Authorization":    []string{"bearer fake-token"},
					"Impersonate-User": []string{"jjackson"},
					"Impersonate-Uid":  []string{"bar"},
				},
			},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: &authenticator.Response{
					User: &user.DefaultInfo{
						Name:   "mmosley",
						Groups: []string{"group1"},
					},
				},
				pass: true,
				err:  nil,
			},
			expCode: http.StatusForbidden,
			expBody: "mmosley is not allowed to impersonate uid 'bar'",
		},

		"an authed request with impersonation groups missing user should fail": {
			req: &http.Request{
				Header: http.Header{
					"Authorization":      []string{"bearer fake-token"},
					"Impersonate-Groups": []string{"bar"},
				},
			},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: &authenticator.Response{
					User: &user.DefaultInfo{
						Name:   "mmosley",
						Groups: []string{"group1"},
					},
				},
				pass: true,
				err:  nil,
			},
			expCode: http.StatusInternalServerError,
			expBody: "no Impersonation-User header found for request",
		},

		"an authed request with impersonation extra missing user should fail": {
			req: &http.Request{
				Header: http.Header{
					"Authorization":         []string{"bearer fake-token"},
					"Impersonate-Extra-foo": []string{"bar"},
				},
			},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: &authenticator.Response{
					User: &user.DefaultInfo{
						Name:   "mmosley",
						Groups: []string{"group1"},
					},
				},
				pass: true,
				err:  nil,
			},
			expCode: http.StatusInternalServerError,
			expBody: "no Impersonation-User header found for request",
		},

		"an authed request with impersonation uid missing user should fail": {
			req: &http.Request{
				Header: http.Header{
					"Authorization":   []string{"bearer fake-token"},
					"Impersonate-Uid": []string{"bar"},
				},
			},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: &authenticator.Response{
					User: &user.DefaultInfo{
						Name:   "mmosley",
						Groups: []string{"group1"},
					},
				},
				pass: true,
				err:  nil,
			},
			expCode: http.StatusInternalServerError,
			expBody: "no Impersonation-User header found for request",
		},

		"an authed request with an invalid impersonation header should fail": {
			req: &http.Request{
				Header: http.Header{
					"Authorization":        []string{"bearer fake-token"},
					"Impersonate-User":     []string{"jjackson"},
					"Impersonate-Not-Real": []string{"bar"},
				},
			},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: &authenticator.Response{
					User: &user.DefaultInfo{
						Name:   "mmosley",
						Groups: []string{"group1"},
					},
				},
				pass: true,
				err:  nil,
			},
			expCode: http.StatusInternalServerError,
			expBody: "",
		},

		// END IMPERSONATION TESTS

		"an authed request with no username is token should 403": {
			req: &http.Request{
				Header: http.Header{
					"Authorization": []string{"bearer fake-token"},
				},
			},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: &authenticator.Response{
					User: &user.DefaultInfo{Name: ""},
				},
				pass: true,
				err:  nil,
			},
			expCode: http.StatusForbidden,
			expBody: "Username claim not available in OIDC Issuer response",
		},
		"an authed request with user should 200": {
			req: &http.Request{
				Header: http.Header{
					"Authorization": []string{"bearer fake-token"},
				},
			},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: &authenticator.Response{
					User: &user.DefaultInfo{Name: "a-user"},
				},
				pass: true,
				err:  nil,
			},
			expCode:  http.StatusOK,
			expBody:  "",
			expUser:  "a-user",
			expGroup: []string{"system:authenticated"},
		},
		"an authed request with user, group, extra should 200": {
			req: &http.Request{
				Header: http.Header{
					"Authorization": []string{"bearer fake-token"},
				},
			},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: &authenticator.Response{
					User: &user.DefaultInfo{
						Name:   "a-user",
						Groups: []string{"my-group"},
						Extra: map[string][]string{
							"foo":     []string{"a", "b"},
							"bar":     []string{"c", "d"},
							"foo-bar": []string{"e", "f"},
						},
					},
				},
				pass: true,
				err:  nil,
			},
			expCode:  http.StatusOK,
			expBody:  "",
			expUser:  "a-user",
			expGroup: []string{"my-group", "system:authenticated"},
			expExtra: map[string][]string{
				"Impersonate-Extra-Foo":     []string{"a", "b"},
				"Impersonate-Extra-Bar":     []string{"c", "d"},
				"Impersonate-Extra-Foo-Bar": []string{"e", "f"},
			},
		},
		"an authed request with user, group, extra but disabled impersonation should return no impersonation and should 200": {
			req: &http.Request{
				Header: http.Header{
					"Authorization": []string{"bearer fake-token"},
				},
			},
			expAuthToken: "fake-token",
			authResponse: &authResponse{
				resp: &authenticator.Response{
					User: &user.DefaultInfo{
						Name:   "a-user",
						Groups: []string{"my-group"},
						Extra: map[string][]string{
							"foo":     []string{"a", "b"},
							"bar":     []string{"c", "d"},
							"foo-bar": []string{"e", "f"},
						},
					},
				},
				pass: true,
				err:  nil,
			},
			config: &Config{
				DisableImpersonation: true,
			},
			expCode:  http.StatusOK,
			expBody:  "",
			expUser:  "",
			expGroup: nil,
			expExtra: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p := newTestProxy(t)

			w := httptest.NewRecorder()

			if test.authResponse != nil {
				p.fakeToken.EXPECT().AuthenticateToken(gomock.Any(), test.expAuthToken).Return(
					test.authResponse.resp, test.authResponse.pass, test.authResponse.err)
			}

			p.fakeRT.expUser = test.expUser
			p.fakeRT.expGroup = test.expGroup
			p.fakeRT.expExtra = test.expExtra
			p.fakeRT.expUid = test.expUid

			if test.config != nil {
				p.config = test.config
			}

			var handler http.Handler
			handler = http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				_, err := p.RoundTrip(req)
				if err != nil {
					t.Errorf("unexpected error: %s", err)
					t.FailNow()
				}
			})

			test.req.URL = new(url.URL)

			handler = p.withHandlers(handler)
			handler.ServeHTTP(w, test.req)

			resp := w.Result()

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("unexpected error: %s", err)
				t.FailNow()
			}

			if test.expBody != strings.TrimSpace(string(body)) {
				t.Errorf("got unexpected response body, exp=%s got=%s",
					test.expBody, body)
			}

			if test.expCode != resp.StatusCode {
				t.Errorf("got unexpected response code, exp=%d got=%d",
					test.expCode, resp.StatusCode)
			}

			p.ctrl.Finish()
		})
	}
}

func TestHeadersConfig(t *testing.T) {
	remoteAddr := "8.8.8.8"

	tests := map[string]struct {
		config   *Config
		expExtra map[string][]string
	}{
		"if no extra headers set or client IP enabled then expect no extras": {
			config: &Config{
				ExtraUserHeaders:                nil,
				ExtraUserHeadersClientIPEnabled: false,
			},
			expExtra: nil,
		},
		"if extra headers set but no client IP enabled then should return added extras": {
			config: &Config{
				ExtraUserHeaders: map[string][]string{
					"foo": []string{"a", "b"},
					"bar": []string{"c", "d", "e"},
				},
				ExtraUserHeadersClientIPEnabled: false,
			},
			expExtra: map[string][]string{
				"Impersonate-Extra-Foo": []string{"a", "b"},
				"Impersonate-Extra-Bar": []string{"c", "d", "e"},
			},
		},
		"if no extra headers set but client IP enabled then should return added client IP": {
			config: &Config{
				ExtraUserHeaders:                nil,
				ExtraUserHeadersClientIPEnabled: true,
			},
			expExtra: map[string][]string{
				"Impersonate-Extra-Remote-Client-Ip": []string{"8.8.8.8"},
			},
		},
		"if extra headers set and client IP enabled then should return extra headers and client IP": {
			config: &Config{
				ExtraUserHeaders: map[string][]string{
					"foo": []string{"a", "b"},
					"bar": []string{"c", "d", "e"},
				},
				ExtraUserHeadersClientIPEnabled: true,
			},
			expExtra: map[string][]string{
				"Impersonate-Extra-Foo":              []string{"a", "b"},
				"Impersonate-Extra-Bar":              []string{"c", "d", "e"},
				"Impersonate-Extra-Remote-Client-Ip": []string{"8.8.8.8"},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p := newTestProxy(t)

			p.config = test.config
			w := httptest.NewRecorder()

			req := &http.Request{
				Header: http.Header{
					"Authorization": []string{"bearer fake-token"},
				},
				RemoteAddr: remoteAddr,
				URL:        new(url.URL),
			}

			authResponse := &authenticator.Response{
				User: &user.DefaultInfo{
					Name:   "a-user",
					Groups: []string{user.AllAuthenticated},
				},
			}

			p.fakeToken.EXPECT().AuthenticateToken(gomock.Any(), "fake-token").Return(authResponse, true, nil)

			p.fakeRT.expUser = "a-user"
			p.fakeRT.expGroup = []string{user.AllAuthenticated}
			p.fakeRT.expExtra = test.expExtra

			var handler http.Handler
			handler = http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				_, err := p.RoundTrip(req)
				if err != nil {
					t.Errorf("unexpected error: %s", err)
					t.FailNow()
				}
			})

			handler = p.withHandlers(handler)
			handler.ServeHTTP(w, req)

			w.Result()

			p.ctrl.Finish()
		})
	}
}
