// Copyright Jetstack Ltd. See LICENSE for details.
package helper

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *Helper) GetServiceAccountSecret(ns, name string) (*corev1.Secret, error) {
	sa, err := h.KubeClient.CoreV1().ServiceAccounts(ns).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	sec, err := h.KubeClient.CoreV1().Secrets(ns).Get(sa.Secrets[0].Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return sec, nil
}
