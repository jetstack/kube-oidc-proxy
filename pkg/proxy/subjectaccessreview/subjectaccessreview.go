// Copyright Jetstack Ltd. See LICENSE for details.
package subjectaccessreview

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	v1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clientazv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
)

// stores information about a user
type Subject struct {
	userName  string
	groups    []string
	extraInfo map[string][]string
	uid       string
}

// structure for storing the review data
type SubjectAccessReview struct {
	subjectAccessReviewer   clientazv1.SubjectAccessReviewInterface
	requester               Subject
	target                  Subject
	success                 bool
	impersonateHeadersFound bool
}

// create a new SubjectAccessReview structure
func New(restConfig *rest.Config, requester Subject, target Subject) (*SubjectAccessReview, error) {
	kubeclient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &SubjectAccessReview{
		subjectAccessReviewer:   kubeclient.AuthorizationV1().SubjectAccessReviews(),
		requester:               requester,
		target:                  target,
		success:                 false,
		impersonateHeadersFound: false,
	}, nil
}

// checks the request for impersonation headers, validates that the user is able to perform that impersonation,
// and builds the target object
func (subjectAccessReview *SubjectAccessReview) CheckAuthorizedForImpersonation(req *http.Request) error {

	subjectAccessReview.impersonateHeadersFound = false

	for key, values := range req.Header {
		if strings.HasPrefix(key, "Impersonate-") {
			subjectAccessReview.impersonateHeadersFound = true

			if key == "Impersonate-User" {
				userToImpersonate := values[0]
				result, err := subjectAccessReview.checkRbacImpersonationAuthorization("users", userToImpersonate)
				if err != nil {
					return err
				} else {
					if !result {
						subjectAccessReview.success = false
						subjectAccessReview.target = Subject{}
						return fmt.Errorf("%s is not allowed to impersonate user '%s'", subjectAccessReview.requester.userName, userToImpersonate)
					} else {
						subjectAccessReview.target.userName = userToImpersonate
					}
				}
			} else if key == "Impersonate-Group" {

				for i := range values {
					groupName := values[i]
					result, err := subjectAccessReview.checkRbacImpersonationAuthorization("groups", groupName)
					if err != nil {
						return err
					} else {
						if !result {
							subjectAccessReview.success = false
							subjectAccessReview.target = Subject{}
							return fmt.Errorf("%s is not allowed to impersonate group '%s'", subjectAccessReview.requester.userName, groupName)
						} else {
							subjectAccessReview.target.groups = append(subjectAccessReview.target.groups, groupName)
						}
					}
				}
			} else if key == "Impersonate-Uid" {
				uidToImpersonate := values[0]
				result, err := subjectAccessReview.checkRbacImpersonationAuthorization("uids", uidToImpersonate)
				if err != nil {
					return err
				} else {
					if !result {
						subjectAccessReview.success = false
						subjectAccessReview.target = Subject{}
						return fmt.Errorf("%s is not allowed to impersonate uid '%s'", subjectAccessReview.requester.userName, uidToImpersonate)
					} else {
						subjectAccessReview.target.uid = uidToImpersonate
					}
				}
			} else if strings.HasPrefix(key, "Impersonate-Extra-") {
				extraName := key[18:]
				for i := range values {
					result, err := subjectAccessReview.checkRbacImpersonationAuthorization("userextras/"+extraName, values[i])
					if err != nil {
						return err
					} else {
						if !result {
							subjectAccessReview.success = false
							subjectAccessReview.target = Subject{}
							return fmt.Errorf("%s is not allowed to impersonate extra info '%s'='%s'", subjectAccessReview.requester.userName, extraName, values[i])
						} else {
							infoVals, ok := subjectAccessReview.target.extraInfo[extraName]

							if !ok {
								infoVals = make([]string, 0)

							}

							infoVals = append(infoVals, values[i])
							subjectAccessReview.target.extraInfo[extraName] = infoVals
						}
					}
				}
			} else if strings.HasPrefix(key, "Impersonate-") {
				// unkown impersonation header, fail
				subjectAccessReview.success = false
				subjectAccessReview.target = Subject{}
				return fmt.Errorf("unknown impersonation header '%s'", key)
			}

		}

		if subjectAccessReview.impersonateHeadersFound {
			// made it this far and have not errored out, we're successful
			subjectAccessReview.success = true
		}
	}

	return nil
}

// submit a SubjectAccessReview request to the API server to validate that impersonation can occur
func (subjectAccessReview *SubjectAccessReview) checkRbacImpersonationAuthorization(resource string, name string) (bool, error) {
	extras := map[string]v1.ExtraValue{}

	for key, value := range subjectAccessReview.requester.extraInfo {
		extras[key] = value
	}

	clusterSubjectAccessReview := v1.SubjectAccessReview{
		Spec: v1.SubjectAccessReviewSpec{
			User:   subjectAccessReview.requester.userName,
			Groups: subjectAccessReview.requester.groups,
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
