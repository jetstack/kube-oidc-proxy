// Copyright Jetstack Ltd. See LICENSE for details.
package util

import (
	"net/http"
	"testing"
)

func TestParseTokenFromHeader(t *testing.T) {
	tests := map[string]struct {
		req   *http.Request
		token string
		ok    bool
	}{
		"should return !ok if request is nil": {
			req:   nil,
			token: "",
			ok:    false,
		},
		"should return !ok if request.Header is nil": {
			req: &http.Request{
				Header: nil,
			},
			token: "",
			ok:    false,
		},
		"should return !ok if no Authorization header given": {
			req: &http.Request{
				Header: map[string][]string{
					"Random-Header1": []string{"foo bar"},
					"Random-Header2": []string{"boo koo"},
				},
			},
			token: "",
			ok:    false,
		},
		"should return !ok if Authorization header is empty": {
			req: &http.Request{
				Header: map[string][]string{
					"Random-Header1": []string{"foo bar"},
					"Authorization":  []string{},
				},
			},
			token: "",
			ok:    false,
		},
		"should return !ok if Authorization header is only 'bearer'": {
			req: &http.Request{
				Header: map[string][]string{
					"Random-Header1": []string{"foo bar"},
					"Authorization":  []string{"bearer"},
				},
			},
			token: "",
			ok:    false,
		},
		"should return !ok if Authorization header is only 'bearertoken'": {
			req: &http.Request{
				Header: map[string][]string{
					"Random-Header1": []string{"foo bar"},
					"Authorization":  []string{"bearertoken"},
				},
			},
			token: "",
			ok:    false,
		},
		"should return 'token' if Authorization header is 'bearer token'": {
			req: &http.Request{
				Header: map[string][]string{
					"Random-Header1": []string{"foo bar"},
					"Authorization":  []string{"bearer token"},
				},
			},
			token: "token",
			ok:    true,
		},
		"should return !ok if Authorization header is 'bearer token' but not first element": {
			req: &http.Request{
				Header: map[string][]string{
					"Random-Header1": []string{"foo bar"},
					"Authorization":  []string{"foo bar", "bearer token"},
				},
			},
			token: "",
			ok:    false,
		},
		"should return 'token' if Authorization header is 'bearer token some-other-string'": {
			req: &http.Request{
				Header: map[string][]string{
					"Random-Header1": []string{"foo bar"},
					"Authorization":  []string{"bearer token some-other-string"},
				},
			},
			token: "token",
			ok:    true,
		},
		"should return 'token' if Authorization header is 'bearer token' but mixed capitals on bearer": {
			req: &http.Request{
				Header: map[string][]string{
					"Random-Header1": []string{"foo bar"},
					"Authorization":  []string{"BeAREr token"},
				},
			},
			token: "token",
			ok:    true,
		},
		"should return !ok if Authorization header is 'bearer token' but the header name is title capitalised": {
			req: &http.Request{
				Header: map[string][]string{
					"Random-Header1": []string{"foo bar"},
					"authorization":  []string{"bearer token"},
				},
			},
			token: "",
			ok:    false,
		},
		"should return !ok if Authorization header has multiple spaces between 'bearer' and 'token'": {
			req: &http.Request{
				Header: map[string][]string{
					"Random-Header1": []string{"foo bar"},
					"Authorization":  []string{"bearer     token"},
				},
			},
			token: "",
			ok:    false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			gToken, gok := ParseTokenFromRequest(test.req)

			if test.ok != gok {
				t.Errorf("unexpected ok, exp=%t got=%t",
					test.ok, gok)
			}

			if test.token != gToken {
				t.Errorf("unexpected token, exp=%s got=%s",
					test.token, gToken)
			}
		})
	}
}
