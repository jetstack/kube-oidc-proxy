// Copyright Jetstack Ltd. See LICENSE for details.
package serviceaccount

import (
	"net/http"
	"testing"
)

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
