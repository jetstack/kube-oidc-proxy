// Copyright Jetstack Ltd. See LICENSE for details.
package getter

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	v1listers "k8s.io/client-go/listers/core/v1"
)

// SAGetter implements ServiceAccountTokenGetter using a clientset.Interface
type SAGetter struct {
	client               clientset.Interface
	secretLister         v1listers.SecretLister
	serviceAccountLister v1listers.ServiceAccountLister
	podLister            v1listers.PodLister
}

// NewGetterFromClient returns a ServiceAccountTokenGetter that
// uses the specified client to retrieve service accounts and secrets.
func NewGetterFromClient(c clientset.Interface, secretLister v1listers.SecretLister,
	serviceAccountLister v1listers.ServiceAccountLister,
	podLister v1listers.PodLister) *SAGetter {
	return &SAGetter{c, secretLister, serviceAccountLister, podLister}
}

func (s *SAGetter) GetServiceAccount(namespace, name string) (*v1.ServiceAccount, error) {
	if serviceAccount, err := s.serviceAccountLister.ServiceAccounts(namespace).Get(name); err == nil {
		return serviceAccount, nil
	}
	return s.client.CoreV1().ServiceAccounts(namespace).Get(name, metav1.GetOptions{})
}

func (s *SAGetter) GetPod(namespace, name string) (*v1.Pod, error) {
	if pod, err := s.podLister.Pods(namespace).Get(name); err == nil {
		return pod, nil
	}
	return s.client.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
}

func (s *SAGetter) GetSecret(namespace, name string) (*v1.Secret, error) {
	if secret, err := s.secretLister.Secrets(namespace).Get(name); err == nil {
		return secret, nil
	}
	return s.client.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
}
