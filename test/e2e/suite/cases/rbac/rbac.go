package rbac

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	rbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jetstack/kube-oidc-proxy/test/e2e/framework"
)

var _ = framework.CasesDescribe("RBAC", func() {
	f := framework.NewDefaultFramework("rbac")

	It("should return with a forbidden request with a valid token without rbac", func() {
		By("Attempting to Get Pods")
		_, err := f.ProxyClient.CoreV1().Pods(f.Namespace.Name).List(metav1.ListOptions{})
		if !k8sErrors.IsForbidden(err) {
			Expect(fmt.Errorf("expected forbidden error, got=%s", err)).NotTo(HaveOccurred())
		}

		By("Attempting to Get Services")
		_, err = f.ProxyClient.CoreV1().Services(f.Namespace.Name).List(metav1.ListOptions{})
		if !k8sErrors.IsForbidden(err) {
			Expect(fmt.Errorf("expected forbidden error, got=%s", err)).NotTo(HaveOccurred())
		}

		By("Attempting to Get Secrets")
		_, err = f.ProxyClient.CoreV1().Secrets(f.Namespace.Name).List(metav1.ListOptions{})
		if !k8sErrors.IsForbidden(err) {
			Expect(fmt.Errorf("expected forbidden error, got=%s", err)).NotTo(HaveOccurred())
		}

		By("Attempting to Get Nodes")
		_, err = f.ProxyClient.CoreV1().Nodes().List(metav1.ListOptions{})
		if !k8sErrors.IsForbidden(err) {
			Expect(fmt.Errorf("expected forbidden error, got=%s", err)).NotTo(HaveOccurred())
		}
	})

	It("should give access to resources based on the group role binding", func() {
		for _, resource := range []string{
			"pods", "services", "secrets",
		} {
			By("Creating Role for Resource " + resource)
			_, err := f.Helper().KubeClient.RbacV1().Roles(f.Namespace.Name).Create(&rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("test-user-role-%s", resource),
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{resource},
						Verbs:     []string{"get", "list"},
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
		}

		By("Creating RoleBinding for Group 'group-1' to access Pods")
		_, err := f.Helper().KubeClient.RbacV1().RoleBindings(f.Namespace.Name).Create(
			&rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-user-binding-group-1-pods",
				},
				Subjects: []rbacv1.Subject{
					{Name: "group-1", Kind: "Group"},
				},
				RoleRef: rbacv1.RoleRef{
					Name: "test-user-role-pods", Kind: "Role"},
			})
		Expect(err).NotTo(HaveOccurred())

		By("Attempting to Get Pods")
		_, err = f.ProxyClient.CoreV1().Pods(f.Namespace.Name).List(metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Attempting to Get Services")
		_, err = f.ProxyClient.CoreV1().Services(f.Namespace.Name).List(metav1.ListOptions{})
		if !k8sErrors.IsForbidden(err) {
			Expect(fmt.Errorf("expected forbidden error, got=%s", err)).NotTo(HaveOccurred())
		}

		By("Attempting to Get Secrets")
		_, err = f.ProxyClient.CoreV1().Secrets(f.Namespace.Name).List(metav1.ListOptions{})
		if !k8sErrors.IsForbidden(err) {
			Expect(fmt.Errorf("expected forbidden error, got=%s", err)).NotTo(HaveOccurred())
		}

		By("Attempting to Get Nodes")
		_, err = f.ProxyClient.CoreV1().Nodes().List(metav1.ListOptions{})
		if !k8sErrors.IsForbidden(err) {
			Expect(fmt.Errorf("expected forbidden error, got=%s", err)).NotTo(HaveOccurred())
		}

		By("Creating RoleBinding for Group 'group-2' to access Services")
		_, err = f.Helper().KubeClient.RbacV1().RoleBindings(f.Namespace.Name).Create(
			&rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-user-binding-group-2-services",
				},
				Subjects: []rbacv1.Subject{
					{Name: "group-2", Kind: "Group"},
				},
				RoleRef: rbacv1.RoleRef{
					Name: "test-user-role-services", Kind: "Role"},
			})
		Expect(err).NotTo(HaveOccurred())

		By("Attempting to Get Pods")
		_, err = f.ProxyClient.CoreV1().Pods(f.Namespace.Name).List(metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Attempting to Get Services")
		_, err = f.ProxyClient.CoreV1().Services(f.Namespace.Name).List(metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Attempting to Get Secrets")
		_, err = f.ProxyClient.CoreV1().Secrets(f.Namespace.Name).List(metav1.ListOptions{})
		if !k8sErrors.IsForbidden(err) {
			Expect(fmt.Errorf("expected forbidden error, got=%s", err)).NotTo(HaveOccurred())
		}

		By("Attempting to Get Nodes")
		_, err = f.ProxyClient.CoreV1().Nodes().List(metav1.ListOptions{})
		if !k8sErrors.IsForbidden(err) {
			Expect(fmt.Errorf("expected forbidden error, got=%s", err)).NotTo(HaveOccurred())
		}

		By("Creating RoleBinding for Group 'group-2' to access Secrets")
		_, err = f.Helper().KubeClient.RbacV1().RoleBindings(f.Namespace.Name).Create(
			&rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-user-binding-group-2-secrets",
				},
				Subjects: []rbacv1.Subject{
					{Name: "group-2", Kind: "Group"},
				},
				RoleRef: rbacv1.RoleRef{
					Name: "test-user-role-secrets", Kind: "Role"},
			})
		Expect(err).NotTo(HaveOccurred())

		By("Attempting to Get Pods")
		_, err = f.ProxyClient.CoreV1().Pods(f.Namespace.Name).List(metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Attempting to Get Services")
		_, err = f.ProxyClient.CoreV1().Services(f.Namespace.Name).List(metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Attempting to Get Secrets")
		_, err = f.ProxyClient.CoreV1().Secrets(f.Namespace.Name).List(metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Attempting to Get Nodes")
		_, err = f.ProxyClient.CoreV1().Nodes().List(metav1.ListOptions{})
		if !k8sErrors.IsForbidden(err) {
			Expect(fmt.Errorf("expected forbidden error, got=%s", err)).NotTo(HaveOccurred())
		}
	})
})
