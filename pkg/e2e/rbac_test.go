// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"fmt"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespaceRbacTest = "kube-oidc-proxy-e2e-rbac"
)

func Test_Rbac(t *testing.T) {
	mustSkipMissingSuite(t)
	mustNamespace(t, namespaceRbacTest)

	rbacclient := e2eSuite.kubeclient.RbacV1()

	validToken := e2eSuite.validToken()

	urlPods := fmt.Sprintf(
		"https://127.0.0.1:%s/api/v1/namespaces/%s/pods",
		e2eSuite.proxyPort,
		namespaceRbacTest,
	)
	urlSvc := fmt.Sprintf(
		"https://127.0.0.1:%s/api/v1/namespaces/%s/services",
		e2eSuite.proxyPort,
		namespaceRbacTest,
	)
	urlSec := fmt.Sprintf(
		"https://127.0.0.1:%s/api/v1/namespaces/%s/secrets",
		e2eSuite.proxyPort,
		namespaceRbacTest,
	)
	urlNodes := fmt.Sprintf("https://127.0.0.1:%s/api/v1/nodes",
		e2eSuite.proxyPort)

	// valid token, no user RBAC should fail Pods
	e2eSuite.testToken(
		t,
		validToken,
		urlPods,
		403,
		`^\{"kind":"Status","apiVersion":"v1","metadata":\{\},"status":"Failure","message":"pods is forbidden: User \\"test-username\\" cannot list (.)+,"reason":"Forbidden","details":\{"kind":"pods"\},"code":403\}$`)

	// valid token, no user RBAC should fail Services
	e2eSuite.testToken(
		t,
		validToken,
		urlSvc,
		403,
		`^\{"kind":"Status","apiVersion":"v1","metadata":\{\},"status":"Failure","message":"services is forbidden: User \\"test-username\\" cannot list (.)+,"reason":"Forbidden","details":\{"kind":"services"\},"code":403\}$`)

	// valid token, no user RBAC should fail Ds
	e2eSuite.testToken(
		t,
		validToken,
		urlSec,
		403,
		`^\{"kind":"Status","apiVersion":"v1","metadata":\{\},"status":"Failure","message":"secrets is forbidden: User \\"test-username\\" cannot list (.)+,"reason":"Forbidden","details":\{"kind":"secrets"\},"code":403\}$`)

	// valid token, no user RBAC should fail Nodes
	e2eSuite.testToken(
		t,
		validToken,
		urlNodes,
		403,
		`^\{"kind":"Status","apiVersion":"v1","metadata":\{\},"status":"Failure","message":"nodes is forbidden: User \\"test-username\\" cannot list (.)+,"reason":"Forbidden","details":\{"kind":"nodes"\},"code":403\}$`)

	// create roles pod, svcs, secrte
	for _, resource := range []string{
		"pods", "services", "secrets",
	} {
		_, err := rbacclient.Roles(namespaceRbacTest).Create(&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-username-role-%s", resource),
				Namespace: namespaceRbacTest,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{resource},
					Verbs:     []string{"get", "list"},
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// group-1 role-binding should give access to pods
	_, err := e2eSuite.kubeclient.RbacV1().RoleBindings(namespaceRbacTest).Create(
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-username-binding-group-1",
				Namespace: namespaceRbacTest,
			},
			Subjects: []rbacv1.Subject{
				{
					Name: "group-1",
					Kind: "Group",
				},
			},
			RoleRef: rbacv1.RoleRef{
				Name: "test-username-role-pods",
				Kind: "Role",
			},
		})
	if err != nil {
		t.Fatal(err)
	}

	// valid token, group RBAC to pods
	e2eSuite.testToken(t, validToken, urlPods, 200, "")
	e2eSuite.testToken(t, validToken, urlSvc, 403, "")
	e2eSuite.testToken(t, validToken, urlSec, 403, "")
	e2eSuite.testToken(t, validToken, urlNodes, 403, "")

	// group-2 role-binding should give access to services
	_, err = e2eSuite.kubeclient.RbacV1().RoleBindings(namespaceRbacTest).Create(
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-username-binding-group-2",
				Namespace: namespaceRbacTest,
			},
			Subjects: []rbacv1.Subject{
				{
					Name: "group-2",
					Kind: "Group",
				},
			},
			RoleRef: rbacv1.RoleRef{
				Name: "test-username-role-services",
				Kind: "Role",
			},
		})
	if err != nil {
		t.Fatal(err)
	}

	// valid token, group RBAC to pods and services
	e2eSuite.testToken(t, validToken, urlPods, 200, "")
	e2eSuite.testToken(t, validToken, urlSvc, 200, "")
	e2eSuite.testToken(t, validToken, urlSec, 403, "")
	e2eSuite.testToken(t, validToken, urlNodes, 403, "")

	// aud-2 role-binding should not give access to secrets
	_, err = e2eSuite.kubeclient.RbacV1().RoleBindings(namespaceRbacTest).Create(
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-username-binding-aud-2",
				Namespace: namespaceRbacTest,
			},
			Subjects: []rbacv1.Subject{
				{
					Name: "aud-2",
					Kind: "Group",
				},
			},
			RoleRef: rbacv1.RoleRef{
				Name: "test-username-role-secrets",
				Kind: "Role",
			},
		})
	if err != nil {
		t.Fatal(err)
	}

	// valid token, group RBAC to pods, svcs
	e2eSuite.testToken(t, validToken, urlPods, 200, "")
	e2eSuite.testToken(t, validToken, urlSvc, 200, "")
	e2eSuite.testToken(t, validToken, urlSec, 403, "")
	e2eSuite.testToken(t, validToken, urlNodes, 403, "")

	// user role-binding should give access to secrets
	_, err = e2eSuite.kubeclient.RbacV1().RoleBindings(namespaceRbacTest).Create(
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-username-binding-test-username",
				Namespace: namespaceRbacTest,
			},
			Subjects: []rbacv1.Subject{
				{
					Name: "test-username",
					Kind: "User",
				},
			},
			RoleRef: rbacv1.RoleRef{
				Name: "test-username-role-secrets",
				Kind: "Role",
			},
		})
	if err != nil {
		t.Fatal(err)
	}

	// valid token, group RBAC to pods, svcs, secrets
	e2eSuite.testToken(t, validToken, urlPods, 200, "")
	e2eSuite.testToken(t, validToken, urlSvc, 200, "")
	e2eSuite.testToken(t, validToken, urlSec, 200, "")
	e2eSuite.testToken(t, validToken, urlNodes, 403, "")
}
