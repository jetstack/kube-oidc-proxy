// Copyright Jetstack Ltd. See LICENSE for details.
package fake

import (
	"context"
	"strings"

	azv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientazv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

var _ clientazv1.SubjectAccessReviewInterface = &FakeReviewer{}

type FakeReviewer struct {
	err error
}

func New(err error) *FakeReviewer {
	return &FakeReviewer{
		err: err,
	}
}

func (f *FakeReviewer) Create(ctx context.Context, req *azv1.SubjectAccessReview, co metav1.CreateOptions) (*azv1.SubjectAccessReview, error) {
	if f.err != nil {
		return nil, f.err
	}

	if req.Spec.ResourceAttributes.Resource == "users" && req.Spec.ResourceAttributes.Name == "jjackson" {
		req.Status = azv1.SubjectAccessReviewStatus{
			Allowed: true,
		}

		return req, nil
	}

	if req.Spec.ResourceAttributes.Resource == "groups" && req.Spec.ResourceAttributes.Name == "group3" {
		req.Status = azv1.SubjectAccessReviewStatus{
			Allowed: true,
		}

		return req, nil
	}

	if req.Spec.ResourceAttributes.Resource == "uids" && req.Spec.ResourceAttributes.Name == "1-2-3-4" {
		req.Status = azv1.SubjectAccessReviewStatus{
			Allowed: true,
		}

		return req, nil
	}

	if strings.ToLower(req.Spec.ResourceAttributes.Resource) == "userextras/remoteaddr" && req.Spec.ResourceAttributes.Name == "1.2.3.4" {
		req.Status = azv1.SubjectAccessReviewStatus{
			Allowed: true,
		}

		return req, nil
	}

	// not an expcted test, or didn't conform to known allowed, fail
	req.Status = azv1.SubjectAccessReviewStatus{
		Allowed: false,
	}

	return req, nil

}
