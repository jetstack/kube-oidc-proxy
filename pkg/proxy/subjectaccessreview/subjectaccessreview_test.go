// Copyright Jetstack Ltd. See LICENSE for details.
package subjectaccessreview

import (
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/subjectaccessreview/fake"
	v1 "k8s.io/api/authorization/v1"
)

// stores the context for each test case
type testT struct {
	// the already authenticated user
	requester Subject

	// the expected target information from the request
	expTarget Subject

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
			requester: Subject{
				userName: "mmosley",
				groups:   []string{"group1", "group2"},
				extraInfo: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
			},

			expTarget: Subject{
				userName: "jjackson",
				groups:   []string{"group3"},
				extraInfo: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
				uid: "1-2-3-4",
			},

			expImpersonationHeaders:  true,
			expAz:                    true,
			expErr:                   nil,
			expErrorRbac:             nil,
			extraImpersonationHeader: false,
		},

		"user not authorized to impersonate target username": {
			requester: Subject{
				userName: "mmosley",
				groups:   []string{"group1", "group2"},
				extraInfo: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
			},

			expTarget: Subject{
				userName: "jjackson-x",
				groups:   []string{"group3"},
				extraInfo: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
				uid: "1-2-3-4",
			},

			expImpersonationHeaders:  true,
			expAz:                    false,
			expErr:                   errors.New("mmosley is not allowed to impersonate user 'jjackson-x'"),
			expErrorRbac:             nil,
			extraImpersonationHeader: false,
		},

		"user not authorized to impersonate target group": {
			requester: Subject{
				userName: "mmosley",
				groups:   []string{"group1", "group2"},
				extraInfo: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
				uid: "1-2-3-4",
			},

			expTarget: Subject{
				userName: "jjackson",
				groups:   []string{"group4"},
				extraInfo: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
				uid: "1-2-3-4",
			},

			expImpersonationHeaders:  true,
			expAz:                    false,
			expErr:                   errors.New("mmosley is not allowed to impersonate group 'group4'"),
			expErrorRbac:             nil,
			extraImpersonationHeader: false,
		},

		"user not authorized to impersonate target extraInfo": {
			requester: Subject{
				userName: "mmosley",
				groups:   []string{"group1", "group2"},
				extraInfo: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
				uid: "1-2-3-4",
			},

			expTarget: Subject{
				userName: "jjackson",
				groups:   []string{"group3"},
				extraInfo: map[string][]string{
					"remoteAddr": []string{"1.2.3.5"},
				},
				uid: "1-2-3-4",
			},

			expImpersonationHeaders:  true,
			expAz:                    false,
			expErr:                   errors.New("mmosley is not allowed to impersonate extra info 'remoteAddr'='1.2.3.5'"),
			expErrorRbac:             nil,
			extraImpersonationHeader: false,
		},

		"user is not authorized to impersonate the uid": {
			requester: Subject{
				userName: "mmosley",
				groups:   []string{"group1", "group2"},
				extraInfo: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
			},

			expTarget: Subject{
				userName: "jjackson",
				groups:   []string{"group3"},
				extraInfo: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
				uid: "1-2-3-5",
			},

			expImpersonationHeaders:  true,
			expAz:                    false,
			expErr:                   errors.New("mmosley is not allowed to impersonate uid '1-2-3-5'"),
			expErrorRbac:             nil,
			extraImpersonationHeader: false,
		},

		"error on the call returns false": {
			requester: Subject{
				userName: "mmosley-x",
				groups:   []string{"group1", "group2"},
				extraInfo: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
			},

			expTarget: Subject{
				userName: "jjackson",
				groups:   []string{"group3"},
				extraInfo: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
				uid: "1-2-3-4",
			},

			expImpersonationHeaders:  true,
			expAz:                    false,
			expErr:                   errors.New("error authorizing the request"),
			expErrorRbac:             errors.New("error authorizing the request"),
			extraImpersonationHeader: false,
		},

		"no impersonation headers found, should set flag as such": {
			requester: Subject{
				userName: "mmosley-x",
				groups:   []string{"group1", "group2"},
				extraInfo: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
			},

			expTarget: Subject{},

			expImpersonationHeaders:  false,
			expAz:                    false,
			expErr:                   nil,
			expErrorRbac:             nil,
			extraImpersonationHeader: false,
		},

		"unknown impersonation header, error": {
			requester: Subject{
				userName: "mmosley-x",
				groups:   []string{"group1", "group2"},
				extraInfo: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
			},

			expTarget: Subject{
				userName: "jjackson",
				groups:   []string{"group3"},
				extraInfo: map[string][]string{
					"remoteAddr": []string{"1.2.3.4"},
				},
				uid: "1-2-3-4",
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

	for key, value := range test.requester.extraInfo {
		extras[key] = value
	}

	testReviewer := &SubjectAccessReview{
		subjectAccessReviewer: fake.New(test.expErrorRbac),
		requester:             test.requester,
		target: Subject{
			extraInfo: make(map[string][]string),
		},
		success: false,
	}

	headers := map[string][]string{}

	if test.expImpersonationHeaders {
		headers["Impersonate-User"] = []string{test.expTarget.userName}
		headers["Impersonate-Group"] = test.expTarget.groups
		headers["Impersonate-Uid"] = []string{test.expTarget.uid}

		for key, value := range test.expTarget.extraInfo {
			headers["Impersonate-Extra-"+key] = value
		}

		if test.extraImpersonationHeader {
			headers["Impersonate-doesnotexist"] = []string{"doesnotmatter"}
		}
	}

	err := testReviewer.CheckAuthorizedForImpersonation(
		&http.Request{
			Header: headers,
		},
	)

	// check if the errors match
	if !reflect.DeepEqual(test.expErr, err) {
		t.Errorf("unexpected error, exp=%t got %t", test.expErr, err)
	}

	//check if impersonation was found when expected
	if test.expImpersonationHeaders != testReviewer.impersonateHeadersFound {
		t.Errorf("unexpected result when checking if impersonation headers were present, exp=%t got=%t", test.expImpersonationHeaders, testReviewer.impersonateHeadersFound)
	}

	// check if authorization matchs
	if testReviewer.success != test.expAz {
		t.Errorf("authorization decision doesn't match, exp=%t got=%t", testReviewer.success, test.expAz)
	}

	// check that the final impersonated user lines up with the expected test case
	if testReviewer.success {
		if !reflect.DeepEqual(test.expTarget, testReviewer.target) {
			t.Errorf(" target doesn't match, exp=%+v got %+v", test.expTarget, testReviewer.target)
		}
	} else {

		if testReviewer.target.userName != "" || testReviewer.target.uid != "" || len(testReviewer.target.groups) > 0 || len(testReviewer.target.extraInfo) > 0 {
			t.Errorf("expected empty target, got=%+v", testReviewer.target)
		}
	}

	// everything checks out!

}
