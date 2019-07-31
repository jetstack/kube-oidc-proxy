// Copyright Jetstack Ltd. See LICENSE for details.
package fake

import (
	authv1 "k8s.io/api/authentication/v1"
)

type FakeReviewer struct {
	CreateFn func(*authv1.TokenReview) (*authv1.TokenReview, error)
}

func New() *FakeReviewer {
	return &FakeReviewer{
		CreateFn: func(*authv1.TokenReview) (*authv1.TokenReview, error) {
			return nil, nil
		},
	}
}

func (f *FakeReviewer) Create(req *authv1.TokenReview) (*authv1.TokenReview, error) {
	return f.CreateFn(req)
}

func (f *FakeReviewer) WithCreate(req *authv1.TokenReview, err error) *FakeReviewer {
	f.CreateFn = func(*authv1.TokenReview) (*authv1.TokenReview, error) {
		return req, err
	}

	return f
}
