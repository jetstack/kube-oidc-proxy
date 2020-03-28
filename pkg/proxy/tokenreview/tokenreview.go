// Copyright Jetstack Ltd. See LICENSE for details.
package tokenreview

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clientauthv1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
	"k8s.io/client-go/rest"

	"github.com/jetstack/kube-oidc-proxy/pkg/util"
)

type TokenReview struct {
	reviewRequester clientauthv1.TokenReviewInterface
	audiences       []string
}

func New(restConfig *rest.Config, audiences []string) (*TokenReview, error) {
	kubeclient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &TokenReview{
		reviewRequester: kubeclient.AuthenticationV1().TokenReviews(),
		audiences:       audiences,
	}, nil
}

func (t *TokenReview) Review(req *http.Request) (bool, error) {
	token, ok := util.ParseTokenFromRequest(req)
	if !ok {
		return false, errors.New("bearer token not found in request")
	}

	review := t.buildReview(token)

	ctx, cancel := context.WithTimeout(req.Context(), time.Second*10)
	defer cancel()

	resp, err := t.reviewRequester.Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}

	if len(resp.Status.Error) > 0 {
		return false, fmt.Errorf("error authenticating using token review: %s",
			resp.Status.Error)
	}

	return resp.Status.Authenticated, nil
}

func (t *TokenReview) buildReview(token string) *authv1.TokenReview {
	return &authv1.TokenReview{
		Spec: authv1.TokenReviewSpec{
			Token:     token,
			Audiences: t.audiences,
		},
	}
}
