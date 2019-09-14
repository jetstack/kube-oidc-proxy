// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func mustSkipMissingSuite(t *testing.T) {
	if e2eSuite == nil {
		t.Skip("e2e suit is nil")
		t.SkipNow()
	}

	if e2eSuite.proxyCmd == nil {
		if err := e2eSuite.runProxy(); err != nil {
			t.Error(err)
			t.FailNow()
		}
	}
}

func mustNamespace(t *testing.T, ns string) {
	_, err := e2eSuite.kubeclient.CoreV1().Namespaces().Get(ns, metav1.GetOptions{})
	if k8sErrors.IsNotFound(err) {

		_, err := e2eSuite.kubeclient.CoreV1().Namespaces().Create(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
			},
		})
		if err != nil {
			t.Error(err)
			t.FailNow()
		}

		return
	}
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
}

func waitForSASecret(t *testing.T, name, namespace string) *corev1.Secret {
	coreclient := e2eSuite.kubeclient.CoreV1()

	for i := 0; i < 10; i++ {
		sa, err := coreclient.ServiceAccounts(namespace).
			Get(name, metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			t.FailNow()
		}

		if len(sa.Secrets) > 0 {
			sec, err := coreclient.Secrets(namespaceTokenPassthroughTest).Get(sa.Secrets[0].Name, metav1.GetOptions{})
			if err != nil {
				t.Error(err)
				t.FailNow()
			}

			return sec
		}

		time.Sleep(time.Second)
	}

	t.Errorf("failed to wait for service account %s/%s to generate secret after 10 seconds",
		namespace, name)
	t.FailNow()

	return nil
}

func mustCreatePodRbac(t *testing.T, name, ns, kind string) {
	_, err := e2eSuite.kubeclient.RbacV1().Roles(ns).Create(&rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-role",
			Namespace: ns,
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
		t.Error(err)
		t.FailNow()
	}

	_, err = e2eSuite.kubeclient.RbacV1().RoleBindings(ns).Create(
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name + "-binding",
				Namespace: ns,
			},
			Subjects: []rbacv1.Subject{
				{
					Name: name,
					Kind: kind,
				},
			},
			RoleRef: rbacv1.RoleRef{
				Name: name + "-role",
				Kind: "Role",
			},
		})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
}

func verifyProxyReadinessPoll(readinessProbeURL string, interval, timeout time.Duration) (bool, error) {
	readinessProbeClient := http.DefaultClient
	err := wait.Poll(interval, timeout, func() (bool, error) {
		resp, err := readinessProbeClient.Get(readinessProbeURL)
		if err != nil {
			return false, fmt.Errorf("failed to verify proxy readiness: %v", err)
		}
		return (resp.StatusCode == http.StatusOK), nil
	})
	return false, err
}
