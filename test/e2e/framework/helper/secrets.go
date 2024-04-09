// Copyright Jetstack Ltd. See LICENSE for details.
package helper

import (
	"context"
	"fmt"

	v1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *Helper) GetServiceAccountSecret(ns, name string) (*corev1.Secret, error) {
	sa, err := h.KubeClient.CoreV1().ServiceAccounts(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var sec *corev1.Secret

	if len(sa.Secrets) > 0 {
		sec, err = h.KubeClient.CoreV1().Secrets(ns).Get(context.TODO(), sa.Secrets[0].Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	} else {
		var requestSeconds int64
		requestSeconds = 600

		// starting in 1.24 ServiceAccounts no longer get Secrets, need to request one bound to a ServiceAccount
		serviceAccountToken, err := h.KubeClient.CoreV1().ServiceAccounts(ns).CreateToken(context.TODO(), sa.Name, &v1.TokenRequest{
			Spec: v1.TokenRequestSpec{
				Audiences:         []string{"https://kubernetes.default.svc.cluster.local"},
				ExpirationSeconds: &requestSeconds,
			},
		}, metav1.CreateOptions{})

		if err != nil {
			return nil, err
		}

		fmt.Printf("TOKEN!!!!! : %v\n", serviceAccountToken.Status.Token)

		secretToReturn := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
			Data: map[string][]byte{corev1.ServiceAccountTokenKey: []byte(serviceAccountToken.Status.Token)},
		}

		return secretToReturn, nil

	}

	return sec, nil
}
