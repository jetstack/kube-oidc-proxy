// Copyright Jetstack Ltd. See LICENSE for details.
package framework

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// CreateKubeNamespace creates a new Kubernetes Namespace for a test.
func (f *Framework) CreateKubeNamespace(baseName string) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("e2e-tests-%v-", baseName),
		},
	}

	return f.KubeClientSet.CoreV1().Namespaces().Create(ns)
}

// DeleteKubeNamespace will delete a namespace resource
func (f *Framework) DeleteKubeNamespace(namespace string) error {
	return f.KubeClientSet.CoreV1().Namespaces().Delete(namespace, nil)
}

// WaitForKubeNamespaceNotExist will wait for the namespace with the given name
// to not exist for up to 2 minutes.
func (f *Framework) WaitForKubeNamespaceNotExist(namespace string) error {
	return wait.PollImmediate(time.Second*2, time.Minute*2, namespaceNotExist(f.KubeClientSet, namespace))
}

func namespaceNotExist(c kubernetes.Interface, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		_, err := c.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	}
}
