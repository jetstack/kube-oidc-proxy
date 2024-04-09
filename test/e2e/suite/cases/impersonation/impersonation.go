// Copyright Jetstack Ltd. See LICENSE for details.
package impersonation

import (
	"context"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	rbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/jetstack/kube-oidc-proxy/test/e2e/framework"
)

var _ = framework.CasesDescribe("Impersonation", func() {
	f := framework.NewDefaultFramework("impersonation")

	It("should allow an authenticated user to impersonate an authorized user when az by rbac", func() {

		By("Creating ClusterRole for user ok-to-impersonate@nodomain.dev to list Pods")
		rolePods, err := f.Helper().KubeClient.RbacV1().ClusterRoles().Create(context.TODO(), &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-user-role-pods-impersonate-",
			},
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list"}},
			},
		}, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Creating ClusterRoleBinding for user ok-to-impersonate@nodomain.dev")
		_, err = f.Helper().KubeClient.RbacV1().ClusterRoleBindings().Create(context.TODO(),
			&rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-user-binding-impersonate",
				},
				Subjects: []rbacv1.Subject{{Name: "ok-to-impersonate@nodomain.dev", Kind: "User"}},
				RoleRef:  rbacv1.RoleRef{Name: rolePods.Name, Kind: "ClusterRole"},
			}, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Impersonating a user, group, and extra")
		tryImpersonationClient(f, rest.ImpersonationConfig{
			UserName: "ok-to-impersonate@nodomain.dev",
			Groups: []string{
				"ok-to-impersonate-group",
			},
			Extra: map[string][]string{
				"oktoimpersonateextra": {
					"foo",
				},
			},
		}, http.StatusOK, "")

	})

	It("should error at proxy when impersonation enabled but a user is not specified", func() {
		By("Impersonating as a group")
		tryImpersonationClient(f, rest.ImpersonationConfig{
			Groups: []string{
				"group-1",
				"group-2",
			},
		}, http.StatusInternalServerError, "no Impersonation-User header found for request")

		By("Impersonating as a extra")
		tryImpersonationClient(f, rest.ImpersonationConfig{
			Extra: map[string][]string{
				"foo": {
					"k1", "k2", "k3",
				},
				"bar": {
					"k1", "k2", "k3",
				},
			},
		}, http.StatusInternalServerError, "no Impersonation-User header found for request")
	})

	It("should return error from proxy when impersonation enabled and impersonation is not authorized by the cluster's RBAC", func() {
		By("Impersonating as a user")
		tryImpersonationClient(f, rest.ImpersonationConfig{
			UserName: "foo@example.com",
		}, http.StatusForbidden, "user@example.com is not allowed to impersonate user 'foo@example.com'")

		By("Impersonating as a user, group")
		tryImpersonationClient(f, rest.ImpersonationConfig{
			UserName: "ok-to-impersonate@nodomain.dev",
			Groups: []string{
				"group-1",
			},
		}, http.StatusForbidden, "user@example.com is not allowed to impersonate group 'group-1'")

		By("Impersonating as a user, extra")
		tryImpersonationClient(f, rest.ImpersonationConfig{
			UserName: "ok-to-impersonate@nodomain.dev",
			Extra: map[string][]string{
				"foo": {
					"k1", "k2", "k3",
				},
			},
		}, http.StatusForbidden, "user@example.com is not allowed to impersonate extra info 'foo'='k1'")

	})

	It("should not error at proxy when impersonation is disabled and impersonation is attempted on a request", func() {
		By("Enabling the disabling of impersonation")
		f.DeployProxyWith(nil, "--disable-impersonation")

		By("Creating ClusterRole for system:anonymous to impersonate")
		roleImpersonate, err := f.Helper().KubeClient.RbacV1().ClusterRoles().Create(context.TODO(), &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-user-role-impersonate-",
			},
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"users"}, Verbs: []string{"impersonate"}},
			},
		}, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Creating Role for user foo to list Pods")
		rolePods, err := f.Helper().KubeClient.RbacV1().Roles(f.Namespace.Name).Create(context.TODO(), &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-user-role-pods-",
			},
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list"}},
			},
		}, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Creating ClusterRoleBinding for user system:anonymous")
		rolebindingImpersonate, err := f.Helper().KubeClient.RbacV1().ClusterRoleBindings().Create(context.TODO(),
			&rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-user-binding-system-anonymous",
				},
				Subjects: []rbacv1.Subject{{Name: "system:anonymous", Kind: "User"}},
				RoleRef:  rbacv1.RoleRef{Name: roleImpersonate.Name, Kind: "ClusterRole"},
			}, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Creating RoleBinding for user foo@example.com")
		rolebindingPods, err := f.Helper().KubeClient.RbacV1().RoleBindings(f.Namespace.Name).Create(context.TODO(),
			&rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-user-binding-user-foo-example-com",
				},
				Subjects: []rbacv1.Subject{{Name: "foo@example.com", Kind: "User"}},
				RoleRef:  rbacv1.RoleRef{Name: rolePods.Name, Kind: "Role"},
			}, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		// build client with impersonation
		config := f.NewProxyRestConfig()
		config.Impersonate = rest.ImpersonationConfig{
			UserName: "foo@example.com",
		}
		client, err := kubernetes.NewForConfig(config)
		Expect(err).NotTo(HaveOccurred())

		// Should not error since we have authorized system:anonymous to
		// impersonate and foo@example.com to list pods
		_, err = client.CoreV1().Pods(f.Namespace.Name).List(context.TODO(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Deleting RoleBinding for user foo@example.com")
		err = f.Helper().KubeClient.RbacV1().RoleBindings(f.Namespace.Name).Delete(context.TODO(), rolebindingPods.Name, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Deleting Role for list Pods")
		err = f.Helper().KubeClient.RbacV1().Roles(f.Namespace.Name).Delete(context.TODO(), rolePods.Name, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Deleting ClusterRoleBinding for user system:anonymous")
		err = f.Helper().KubeClient.RbacV1().ClusterRoleBindings().Delete(context.TODO(), rolebindingImpersonate.Name, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Deleting ClusterRole for Impersonate")
		err = f.Helper().KubeClient.RbacV1().ClusterRoles().Delete(context.TODO(), roleImpersonate.Name, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	})
})

func tryImpersonationClient(f *framework.Framework, impConfig rest.ImpersonationConfig, expectedCode int, expRespBody string) {
	// build client with impersonation
	config := f.NewProxyRestConfig()
	config.Impersonate = impConfig
	client, err := kubernetes.NewForConfig(config)
	Expect(err).NotTo(HaveOccurred())

	var resp string
	var respCode int

	_, err = client.CoreV1().Pods(f.Namespace.Name).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		kErr, ok := err.(*k8sErrors.StatusError)
		if !ok {
			Expect(err).NotTo(HaveOccurred())
		}
		respCode = int(kErr.ErrStatus.Code)
		fmt.Printf("http status code %d\n", respCode)
		if respCode != http.StatusOK {
			if len(kErr.Status().Details.Causes) > 0 {
				resp = kErr.Status().Details.Causes[0].Message
			} else {
				resp = kErr.Error()
			}
		} else {
			resp = ""
		}

	} else {
		respCode = http.StatusOK
	}

	// check body and status code the token was rejected
	//if int(kErr.Status().Code) != http.StatusForbidden ||

	if respCode != expectedCode {
		Expect(fmt.Errorf("expected status code=%d, got=%d resp=%s", expectedCode, respCode, resp)).NotTo(HaveOccurred())
	}

	if resp != expRespBody {
		Expect(fmt.Errorf("expected response=%s, got=%s", expRespBody, resp)).NotTo(HaveOccurred())
	}

	/*if int(kErr.Status().Code) != expectedCode ||
		resp != expRespBody {
		Expect(fmt.Errorf("expected status code %d with body \"%s\", got code=%d, body=\"%s\"",
			http.StatusForbidden, expRespBody, int(kErr.Status().Code), resp)).NotTo(HaveOccurred())
	}*/
}
