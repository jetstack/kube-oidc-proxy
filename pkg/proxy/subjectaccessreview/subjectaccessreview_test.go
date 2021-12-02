// Copyright Jetstack Ltd. See LICENSE for details.
package subjectaccessreview

import (
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/subjectaccessreview/fake"
	v1 "k8s.io/api/authorization/v1"
	"k8s.io/apiserver/pkg/authentication/user"
)

// stores the context for each test case
type testT struct {
	// the already authenticated user
	requester user.Info

	// the expected target information from the request
	expTarget user.Info

	// the expected authorization decision
	expAz bool

	// the expected error
	expErr error

	// expected error from rbacCheck
	expErrorRbac error

	// should the impersonation headers be found?
	expImpersonationHeaders bool

	// should include extra impersonation header?
	extraImpersonationHeader bool
}

func TestSubectAccessReview(t *testing.T) {
	tests := map[string]testT{
		"if all reviews pass, user is authorized to impersonate": {
			requester: &user.DefaultInfo{
				Name:   "mmosley",
				Groups: []string{"group1", "group2"},
				Extra: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
			},

			expTarget: &user.DefaultInfo{
				Name:   "jjackson",
				Groups: []string{"group3"},
				Extra: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
				UID: "1-2-3-4",
			},

			expImpersonationHeaders:  true,
			expAz:                    true,
			expErr:                   nil,
			expErrorRbac:             nil,
			extraImpersonationHeader: false,
		},

		"user not authorized to impersonate target username": {
			requester: &user.DefaultInfo{
				Name:   "mmosley",
				Groups: []string{"group1", "group2"},
				Extra: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
			},

			expTarget: &user.DefaultInfo{
				Name:   "jjackson-x",
				Groups: []string{"group3"},
				Extra: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
				UID: "1-2-3-4",
			},

			expImpersonationHeaders:  true,
			expAz:                    false,
			expErr:                   errors.New("mmosley is not allowed to impersonate user 'jjackson-x'"),
			expErrorRbac:             nil,
			extraImpersonationHeader: false,
		},

		"user not authorized to impersonate target group": {
			requester: &user.DefaultInfo{
				Name:   "mmosley",
				Groups: []string{"group1", "group2"},
				Extra: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
				UID: "1-2-3-4",
			},

			expTarget: &user.DefaultInfo{
				Name:   "jjackson",
				Groups: []string{"group4"},
				Extra: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
				UID: "1-2-3-4",
			},

			expImpersonationHeaders:  true,
			expAz:                    false,
			expErr:                   errors.New("mmosley is not allowed to impersonate group 'group4'"),
			expErrorRbac:             nil,
			extraImpersonationHeader: false,
		},

		"user not authorized to impersonate target extraInfo": {
			requester: &user.DefaultInfo{
				Name:   "mmosley",
				Groups: []string{"group1", "group2"},
				Extra: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
				UID: "1-2-3-4",
			},

			expTarget: &user.DefaultInfo{
				Name:   "jjackson",
				Groups: []string{"group3"},
				Extra: map[string][]string{
					"remoteAddr": []string{"1.2.3.5"},
				},
				UID: "1-2-3-4",
			},

			expImpersonationHeaders:  true,
			expAz:                    false,
			expErr:                   errors.New("mmosley is not allowed to impersonate extra info 'remoteAddr'='1.2.3.5'"),
			expErrorRbac:             nil,
			extraImpersonationHeader: false,
		},

		"user is not authorized to impersonate the uid": {
			requester: &user.DefaultInfo{
				Name:   "mmosley",
				Groups: []string{"group1", "group2"},
				Extra: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
			},

			expTarget: &user.DefaultInfo{
				Name:   "jjackson",
				Groups: []string{"group3"},
				Extra: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
				UID: "1-2-3-5",
			},

			expImpersonationHeaders:  true,
			expAz:                    false,
			expErr:                   errors.New("mmosley is not allowed to impersonate uid '1-2-3-5'"),
			expErrorRbac:             nil,
			extraImpersonationHeader: false,
		},

		"error on the call returns false": {
			requester: &user.DefaultInfo{
				Name:   "mmosley-x",
				Groups: []string{"group1", "group2"},
				Extra: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
			},

			expTarget: &user.DefaultInfo{
				Name:   "jjackson",
				Groups: []string{"group3"},
				Extra: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
				UID: "1-2-3-4",
			},

			expImpersonationHeaders:  true,
			expAz:                    false,
			expErr:                   errors.New("error authorizing the request"),
			expErrorRbac:             errors.New("error authorizing the request"),
			extraImpersonationHeader: false,
		},

		"no impersonation headers found, should set flag as such": {
			requester: &user.DefaultInfo{
				Name:   "mmosley-x",
				Groups: []string{"group1", "group2"},
				Extra: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
			},

			expTarget: &user.DefaultInfo{},

			expImpersonationHeaders:  false,
			expAz:                    false,
			expErr:                   nil,
			expErrorRbac:             nil,
			extraImpersonationHeader: false,
		},

		"unknown impersonation header, error": {
			requester: &user.DefaultInfo{
				Name:   "mmosley-x",
				Groups: []string{"group1", "group2"},
				Extra: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
			},

			expTarget: &user.DefaultInfo{
				Name:   "jjackson",
				Groups: []string{"group3"},
				Extra: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
				UID: "1-2-3-4",
			},

			expImpersonationHeaders:  true,
			expAz:                    false,
			expErr:                   errors.New("unknown impersonation header 'Impersonate-doesnotexist'"),
			expErrorRbac:             nil,
			extraImpersonationHeader: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			runTest(t, name, test)
		})
	}
}

func runTest(t *testing.T, name string, test testT) {

	extras := map[string]v1.ExtraValue{}

	for key, value := range test.requester.GetExtra() {
		extras[key] = value
	}

	testReviewer, _ := New(fake.New(test.expErrorRbac))

	headers := map[string][]string{}

	if test.expImpersonationHeaders {
		headers["Impersonate-User"] = []string{test.expTarget.GetName()}
		headers["Impersonate-Group"] = test.expTarget.GetGroups()
		headers["Impersonate-Uid"] = []string{test.expTarget.GetUID()}

		for key, value := range test.expTarget.GetExtra() {
			headers["Impersonate-Extra-"+key] = value
		}

		if test.extraImpersonationHeader {
			headers["Impersonate-doesnotexist"] = []string{"doesnotmatter"}
		}
	}

	target, err := testReviewer.CheckAuthorizedForImpersonation(
		&http.Request{
			Header: headers,
		}, test.requester)

	// check if the errors match
	if !reflect.DeepEqual(test.expErr, err) {
		t.Errorf("unexpected error, exp=%t got %t", test.expErr, err)
	}

	//check if impersonation was found when expected

	headersFound := !(err == nil && target == nil)

	if test.expImpersonationHeaders != headersFound {
		t.Errorf("unexpected result when checking if impersonation headers were present, exp=%t got=%t", test.expImpersonationHeaders, (err == nil && target == nil))
	}

	azSuccess := target != nil && err == nil
	// check if authorization matchs
	if azSuccess != test.expAz {
		t.Errorf("authorization decision doesn't match, exp=%t got=%t", azSuccess, test.expAz)
	}

	// check that the final impersonated user lines up with the expected test case
	if azSuccess {
		if !reflect.DeepEqual(test.expTarget, target) {
			t.Errorf(" target doesn't match, exp=%+v got %+v", test.expTarget, target)
		}
	} else {

		if target != nil {
			t.Errorf("expected empty target, got=%+v", target)
		}
	}

	// everything checks out!

}
