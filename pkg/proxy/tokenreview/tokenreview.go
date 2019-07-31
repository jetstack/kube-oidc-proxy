// Copyright Jetstack Ltd. See LICENSE for details.
package tokenreview

import (
	"fmt"

	authv1 "k8s.io/api/authentication/v1"
	"k8s.io/client-go/kubernetes"
	clientauthv1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
	"k8s.io/client-go/rest"
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

func (t *TokenReview) Review(token string) (bool, error) {
	review := t.buildReview(token)

	resp, err := t.reviewRequester.Create(review)
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
