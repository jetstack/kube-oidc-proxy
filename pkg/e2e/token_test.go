// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespaceTokenTest = "kube-oidc-proxy-e2e-token"
)

func Test_Token(t *testing.T) {
	if e2eSuite == nil {
		t.Skip("e2eSuite not defined")
		return
	}

	defer func() {
		err := e2eSuite.kubeclient.Core().Namespaces().Delete(namespaceTokenTest, nil)
		if err != nil {
			t.Errorf("failed to delete test namespace: %s", err)
		}
	}()

	_, err := e2eSuite.kubeclient.Core().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceTokenTest,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = e2eSuite.kubeclient.Rbac().Roles(namespaceTokenTest).Create(&rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-username-role",
			Namespace: namespaceTokenTest,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = e2eSuite.kubeclient.Rbac().RoleBindings(namespaceTokenTest).Create(
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-username-binding",
				Namespace: namespaceTokenTest,
			},
			Subjects: []rbacv1.Subject{
				{
					Name: "test-username",
					Kind: "User",
				},
			},
			RoleRef: rbacv1.RoleRef{
				Name: "test-username-role",
				Kind: "Role",
			},
		})
	if err != nil {
		t.Fatal(err)
	}

	url := fmt.Sprintf(
		"https://127.0.0.1:%s/api/v1/namespaces/%s/pods",
		e2eSuite.proxyPort,
		namespaceTokenTest,
	)

	// valid token
	e2eSuite.test(
		t,
		e2eSuite.validToken(),
		url,
		200,
		nil)

	for _, test := range []struct {
		token   []byte
		expCode int
		expBody []byte
	}{

		// no bearer token
		{
			token:   nil,
			expCode: 401,
			expBody: []byte("Unauthorized"),
		},

		//	 invalid bearer token
		{
			token:   []byte("bad-payload"),
			expCode: 401,
			expBody: []byte("Unauthorized"),
		},

		// wrong issuer
		{
			token: []byte(fmt.Sprintf(`{
	"iss":"incorrect-issuer",
	"aud":["kube-oidc-proxy_e2e_client-id","aud-2"],
	"e2e-username-claim":"test-username",
	"e2e-groups-claim":["group-1","group-2"],
	"exp":%d
	}`, time.Now().Add(time.Minute).Unix())),
			expCode: 401,
			expBody: []byte("Unauthorized"),
		},

		// no audience
		{
			token: []byte(fmt.Sprintf(`{
	"iss":"https://127.0.0.1:%s",
	"aud":[],
	"e2e-username-claim":"test-username",
	"e2e-groups-claim":["group-1","group-2"],
	"exp":%d
	}`, e2eSuite.issuer.Port(), time.Now().Add(time.Minute).Unix())),
			expCode: 401,
			expBody: []byte("Unauthorized"),
		},

		// wrong audience
		{
			token: []byte(fmt.Sprintf(`{
	"iss":"https://127.0.0.1:%s",
	"aud":["aud-1", "aud-2"],
	"e2e-username-claim":"test-username",
	"e2e-groups-claim":["group-1","group-2"],
	"exp":%d
	}`, e2eSuite.issuer.Port(), time.Now().Add(time.Minute).Unix())),
			expCode: 401,
			expBody: []byte("Unauthorized"),
		},

		// token expires now
		{
			token: []byte(fmt.Sprintf(`{
	"iss":"https://127.0.0.1:%s",
	"aud":["kube-oidc-proxy_e2e_client-id","aud-2"],
	"e2e-username-claim":"test-username",
	"e2e-groups-claim":["group-1","group-2"],
	"exp":%d
	}`, e2eSuite.issuer.Port(), time.Now().Unix())),
			expCode: 401,
			expBody: []byte("Unauthorized"),
		},
	} {
		e2eSuite.test(t, test.token, url, test.expCode, test.expBody)
	}
}
