// Copyright Jetstack Ltd. See LICENSE for details.
package subjectaccessreview

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	v1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/kubernetes"
	clientazv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
)

// structure for storing the review data
type SubjectAccessReview struct {
	subjectAccessReviewer clientazv1.SubjectAccessReviewInterface
}

// create a new SubjectAccessReview structure
func New(restConfig *rest.Config, requester user.Info, target user.Info) (*SubjectAccessReview, error) {
	kubeclient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &SubjectAccessReview{
		subjectAccessReviewer: kubeclient.AuthorizationV1().SubjectAccessReviews(),
	}, nil
}

// checks the request for impersonation headers, validates that the user is able to perform that impersonation,
// and builds the target object
func (subjectAccessReview *SubjectAccessReview) CheckAuthorizedForImpersonation(req *http.Request, requester user.Info) (user.Info, error) {

	hasImpersonation := false

	targetUser := &user.DefaultInfo{
		Name:   "",
		Groups: make([]string, 0),
		Extra:  map[string][]string{},
		UID:    "",
	}

	for key, values := range req.Header {
		keyToCheck := strings.ToLower(key)
		if strings.HasPrefix(keyToCheck, "impersonate-") {
			hasImpersonation = true
			if keyToCheck == "impersonate-user" {
				userToImpersonate := values[0]
				result, err := subjectAccessReview.checkRbacImpersonationAuthorization("users", userToImpersonate, requester)
				if err != nil {
					return nil, err
				} else {
					if !result {
						return nil, fmt.Errorf("%s is not allowed to impersonate user '%s'", requester.GetName(), userToImpersonate)
					} else {
						targetUser.Name = userToImpersonate
					}
				}
			} else if keyToCheck == "impersonate-group" {

				for i := range values {
					groupName := values[i]
					result, err := subjectAccessReview.checkRbacImpersonationAuthorization("groups", groupName, requester)
					if err != nil {
						return nil, err
					} else {
						if !result {
							return nil, fmt.Errorf("%s is not allowed to impersonate group '%s'", requester.GetName(), groupName)
						} else {
							targetUser.Groups = append(targetUser.Groups, groupName)
						}
					}
				}
			} else if keyToCheck == "impersonate-uid" {
				uidToImpersonate := values[0]
				result, err := subjectAccessReview.checkRbacImpersonationAuthorization("uids", uidToImpersonate, requester)
				if err != nil {
					return nil, err
				} else {
					if !result {
						return nil, fmt.Errorf("%s is not allowed to impersonate uid '%s'", requester.GetName(), uidToImpersonate)
					} else {
						targetUser.UID = uidToImpersonate
					}
				}
			} else if strings.HasPrefix(keyToCheck, "impersonate-extra-") {
				extraName := key[18:]
				for i := range values {
					result, err := subjectAccessReview.checkRbacImpersonationAuthorization("userextras/"+extraName, values[i], requester)
					if err != nil {
						return nil, err
					} else {
						if !result {

							return nil, fmt.Errorf("%s is not allowed to impersonate extra info '%s'='%s'", requester.GetName(), extraName, values[i])
						} else {
							infoVals, ok := targetUser.Extra[extraName]

							if !ok {
								infoVals = make([]string, 0)

							}

							infoVals = append(infoVals, values[i])
							targetUser.Extra[extraName] = infoVals
						}
					}
				}
			} else if strings.HasPrefix(keyToCheck, "impersonate-") {
				// unkown impersonation header, fail
				return nil, fmt.Errorf("unknown impersonation header '%s'", key)
			}

		}

	}

	if hasImpersonation {
		//haven't errored out, but has impersonation - returning target user
		return targetUser, nil
	} else {
		//no impersonation, no user to return
		return nil, nil
	}
}

// submit a SubjectAccessReview request to the API server to validate that impersonation can occur
func (subjectAccessReview *SubjectAccessReview) checkRbacImpersonationAuthorization(resource string, name string, requester user.Info) (bool, error) {
	extras := map[string]v1.ExtraValue{}

	for key, value := range requester.GetExtra() {
		extras[key] = value
	}

	clusterSubjectAccessReview := v1.SubjectAccessReview{
		Spec: v1.SubjectAccessReviewSpec{
			User:   requester.GetName(),
			Groups: requester.GetGroups(),
			Extra:  extras,
			ResourceAttributes: &v1.ResourceAttributes{
				Verb:     "impersonate",
				Group:    "",
				Resource: resource,
				Name:     name,
			},
		},
	}

	reviewResult, err := subjectAccessReview.subjectAccessReviewer.Create(context.TODO(), &clusterSubjectAccessReview, metav1.CreateOptions{})

	if err != nil {
		return false, err
	} else {
		return reviewResult.Status.Allowed, nil
	}
}
