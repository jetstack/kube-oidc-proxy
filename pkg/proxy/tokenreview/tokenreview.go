// Copyright Jetstack Ltd. See LICENSE for details.
package tokenreview

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	authv1 "k8s.io/api/authentication/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clientauthv1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
	"k8s.io/client-go/rest"

	"github.com/jetstack/kube-oidc-proxy/pkg/metrics"
	proxycontext "github.com/jetstack/kube-oidc-proxy/pkg/proxy/context"
	"github.com/jetstack/kube-oidc-proxy/pkg/util"
)

var (
	timeout = time.Second * 10
)

type TokenReview struct {
	reviewRequester clientauthv1.TokenReviewInterface
	metrics         *metrics.Metrics
	audiences       []string
}

func New(restConfig *rest.Config, metrics *metrics.Metrics, audiences []string) (*TokenReview, error) {
	kubeclient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &TokenReview{
		reviewRequester: kubeclient.AuthenticationV1().TokenReviews(),
		metrics:         metrics,
		audiences:       audiences,
	}, nil
}

func (t *TokenReview) Review(req *http.Request) (bool, error) {
	var (
		code          int
		user          string
		authenticated bool
		err           error
	)

	// Start clock on metrics
	tokenReviewDuration := time.Now()
	req, remoteAddr := proxycontext.RemoteAddr(req)

	token, ok := util.ParseTokenFromRequest(req)
	if !ok {
		return false, errors.New("bearer token not found in request")
	}
	review := t.buildReview(token)

	// Setup metrics observation on defer
	defer func() {
		if err != nil {
			if status := apierrors.APIStatus(nil); errors.As(err, &status) {
				code = int(status.Status().Code)
			}
		}

		t.metrics.ObserveTokenReivewLookup(authenticated, code, remoteAddr, user, time.Since(tokenReviewDuration))
	}()

	ctx, cancel := context.WithTimeout(req.Context(), timeout)
	defer cancel()

	var resp *authv1.TokenReview
	resp, err = t.reviewRequester.Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}

	// Since no error to the API server for token review, we have 200 response
	// code.
	code = 200
	user = resp.Status.User.Username
	authenticated = resp.Status.Authenticated

	if len(resp.Status.Error) > 0 {
		return false, fmt.Errorf("error authenticating using token review: %s",
			resp.Status.Error)
	}

	return authenticated, nil
}

func (t *TokenReview) buildReview(token string) *authv1.TokenReview {
	return &authv1.TokenReview{
		Spec: authv1.TokenReviewSpec{
			Token:     token,
			Audiences: t.audiences,
		},
	}
}
