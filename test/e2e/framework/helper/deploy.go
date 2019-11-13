package helper

import (
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/jetstack/kube-oidc-proxy/pkg/util"
)

const (
	IssuerName = "oidc-issuer-e2e"
	ProxyName  = "kube-oidc-proxy-e2e"

	ProxyClientID = "kube-oidc-proxy-e2e-client_id"
)

func (h *Helper) DeployProxy(ns string, oidcKeyBundle *util.KeyBundle) (*util.KeyBundle, error) {
	cnt := corev1.Container{
		Image: "kube-oidc-proxy-e2e",
		Args: []string{
			"--secure-port=6443",
			"--tls-cert-file=/tls/crt.pem",
			"--tls-private-key-file=/key.pem",
			fmt.Sprintf("--oidc-client-id=%s", ProxyClientID),
			fmt.Sprintf("--oidc-issuer-url=https://oidc-issuer-e2e.%s.cluster.local", ns),
			"--oidc-username-claim=email",
			"--oidc-ca-file=/oidc/ca.pem",
		},
		VolumeMounts: []corev1.VolumeMount{
			corev1.VolumeMount{
				MountPath: "/tls",
				Name:      "tls",
				ReadOnly:  true,
			},
			corev1.VolumeMount{
				MountPath: "/oidc",
				Name:      "oidc",
				ReadOnly:  true,
			},
		},
		Ports: []corev1.ContainerPort{
			corev1.ContainerPort{
				ContainerPort: 6443,
			},
		},
	}

	volume := corev1.Volume{
		Name: "oidc",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "oidc-ca",
			},
		},
	}

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oidc-ca",
			Namespace: ns,
		},
		Data: map[string][]byte{
			"ca.pem": oidcKeyBundle.CertBytes,
		},
	}

	_, err := h.KubeClient.CoreV1().Secrets(ns).Create(sec)
	if err != nil {
		return nil, err
	}

	return h.deployApp(ns, IssuerName, cnt, volume)
}

func (h *Helper) DeployIssuer(ns string) (*util.KeyBundle, error) {
	cnt := corev1.Container{
		Image: "oidc-issuer-e2e",
		Args: []string{
			"--secure-port=6443",
			fmt.Sprintf("--issuer-url=https://oidc-issuer-e2e.%s.cluster.local", ns),
			"--tls-cert-file=/tls/cert.pem",
			"--tls-private-key-file=/tls/key.pem",
		},
		VolumeMounts: []corev1.VolumeMount{
			corev1.VolumeMount{
				MountPath: "/tls",
				Name:      "tls",
				ReadOnly:  true,
			},
		},
		Ports: []corev1.ContainerPort{
			corev1.ContainerPort{
				ContainerPort: 6443,
			},
		},
	}

	return h.deployApp(ns, IssuerName, cnt)
}

func (h *Helper) deployApp(ns, name string, container corev1.Container, volumes ...corev1.Volume) (*util.KeyBundle, error) {
	keyBundle, err := util.NewTLSSelfSignedCertKey(os.TempDir(), "oidc-issuer-e2e")
	if err != nil {
		return nil, err
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:       6443,
					Protocol:   "TCP",
					TargetPort: intstr.FromInt(6443),
				},
			},
			Type: "ClusterIP",
			Selector: map[string]string{
				"app": name,
			},
		},
	}

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: map[string][]byte{
			corev1.TLSCertKey:       keyBundle.CertBytes,
			corev1.TLSPrivateKeyKey: keyBundle.KeyBytes,
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				"app": name,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{container},
			Volumes: append(volumes,
				corev1.Volume{
					Name: "tls",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: name,
						},
					},
				},
			),
		},
	}

	_, err = h.KubeClient.CoreV1().Services(ns).Create(svc)
	if err != nil {
		return nil, err
	}

	_, err = h.KubeClient.CoreV1().Secrets(ns).Create(sec)
	if err != nil {
		return nil, err
	}

	_, err = h.KubeClient.CoreV1().Pods(ns).Create(pod)
	if err != nil {
		return nil, err
	}

	if err := h.WaitForPodReady(ns, name, time.Second*20); err != nil {
		return nil, err
	}

	return keyBundle, nil
}

func (h *Helper) DeleteIssuer(ns string) error {
	return h.deleteApp(ns, IssuerName)
}
func (h *Helper) DeleteProxy(ns string) error {
	return h.deleteApp(ns, ProxyName)
}

func (h *Helper) deleteApp(ns, name string) error {
	err := h.KubeClient.CoreV1().Pods(ns).Delete(name, nil)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	err = h.KubeClient.CoreV1().Secrets(ns).Delete(name, nil)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	err = h.KubeClient.CoreV1().Services(ns).Delete(name, nil)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	if err := h.WaitForPodDeletion(ns, name, time.Second*30); err != nil {
		return err
	}

	return nil
}
