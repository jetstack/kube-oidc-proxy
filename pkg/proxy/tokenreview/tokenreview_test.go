// Copyright Jetstack Ltd. See LICENSE for details.
package tokenreview

import (
	"errors"
	"net/http"
	"reflect"
	"testing"

	authv1 "k8s.io/api/authentication/v1"

	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/tokenreview/fake"
)

type testT struct {
	reviewResp *authv1.TokenReview
	errResp    error

	expAuth bool
	expErr  error
}

func TestReview(t *testing.T) {

	tests := map[string]testT{
		"if a create fails then this error is returned": {
			reviewResp: nil,
			errResp:    errors.New("create error response"),
			expAuth:    false,
			expErr:     errors.New("create error response"),
		},

		"if an error exists in the status of the response pass error back": {
			reviewResp: &authv1.TokenReview{
				Status: authv1.TokenReviewStatus{
					Error: "status error",
				},
			},
			errResp: nil,
			expAuth: false,
			expErr:  errors.New("error authenticating using token review: status error"),
		},

		"if the response returns unauthenticated, return false": {
			reviewResp: &authv1.TokenReview{
				Status: authv1.TokenReviewStatus{
					Authenticated: false,
				},
			},
			errResp: nil,
			expAuth: false,
			expErr:  nil,
		},

		"if the response returns authenticated, return true": {
			reviewResp: &authv1.TokenReview{
				Status: authv1.TokenReviewStatus{
					Authenticated: true,
				},
			},
			errResp: nil,
			expAuth: true,
			expErr:  nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			runTest(t, test)
		})
	}
}

func runTest(t *testing.T, test testT) {
	tReviewer := &TokenReview{
		audiences:       nil,
		reviewRequester: fake.New().WithCreate(test.reviewResp, test.errResp),
	}

	authed, err := tReviewer.Review(
		&http.Request{
			Header: map[string][]string{
				"Authorization": []string{"bearer test-token"},
			},
		},
	)

	if !reflect.DeepEqual(test.expErr, err) {
		t.Errorf("got unexpected error, exp=%v got=%v",
			test.expErr, err)
	}

	if test.expAuth != authed {
		t.Errorf("got unexpected authed, exp=%t got=%t",
			test.expAuth, authed)
	}
}
