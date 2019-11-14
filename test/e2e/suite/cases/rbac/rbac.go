package rbac

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	//corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/jetstack/kube-oidc-proxy/test/e2e/framework"
)

var _ = framework.CasesDescribe("RBAC", func() {
	f := framework.NewDefaultFramework("rbac")

	var rClient *kubernetes.Clientset
	JustBeforeEach(func() {
		config, err := f.NewValidRestConfig()
		Expect(err).NotTo(HaveOccurred())

		rClient, err = kubernetes.NewForConfig(config)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return with a forbidden pod list request with a valid token", func() {
		_, err := rClient.CoreV1().Pods(f.Namespace.Name).List(metav1.ListOptions{})
		if !k8sErrors.IsForbidden(err) {
			Expect(fmt.Errorf("expected forbidden error, got=%s", err)).NotTo(HaveOccurred())
		}
	})

	It("should return with a forbidden service list request with a valid token", func() {
		_, err := rClient.CoreV1().Services(f.Namespace.Name).List(metav1.ListOptions{})
		if !k8sErrors.IsForbidden(err) {
			Expect(fmt.Errorf("expected forbidden error, got=%s", err)).NotTo(HaveOccurred())
		}
	})

	It("should return with a forbidden secret list request with a valid token", func() {
		_, err := rClient.CoreV1().Secrets(f.Namespace.Name).List(metav1.ListOptions{})
		if !k8sErrors.IsForbidden(err) {
			Expect(fmt.Errorf("expected forbidden error, got=%s", err)).NotTo(HaveOccurred())
		}
	})

	It("should return with a forbidden nodes list request with a valid token", func() {
		_, err := rClient.CoreV1().Nodes().List(metav1.ListOptions{})
		if !k8sErrors.IsForbidden(err) {
			Expect(fmt.Errorf("expected forbidden error, got=%s", err)).NotTo(HaveOccurred())
		}
	})
})
