// Copyright Jetstack Ltd. See LICENSE for details.
package helper

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/jetstack/kube-oidc-proxy/test/util"
)

const (
	ProxyName         = "kube-oidc-proxy-e2e"
	IssuerName        = "oidc-issuer-e2e"
	FakeAPIServerName = "fake-apiserver-e2e"
)

func (h *Helper) DeployProxy(ns *corev1.Namespace, issuerURL *url.URL, clientID string,
	oidcKeyBundle *util.KeyBundle, extraVolumes []corev1.Volume, extraArgs ...string) (*util.KeyBundle, *url.URL, error) {
	cnt := corev1.Container{
		Name:            ProxyName,
		Image:           ProxyName,
		ImagePullPolicy: corev1.PullNever,
		Args: append([]string{
			"kube-oidc-proxy",
			"--secure-port=6443",
			"--tls-cert-file=/tls/cert.pem",
			"--tls-private-key-file=/tls/key.pem",
			fmt.Sprintf("--oidc-client-id=%s", clientID),
			fmt.Sprintf("--oidc-issuer-url=%s", issuerURL),
			"--oidc-username-claim=email",
			"--oidc-groups-claim=groups",
			"--oidc-ca-file=/oidc/ca.pem",
			"--oidc-ca-file=/oidc/ca.pem",
			"--v=10",
		}, extraArgs...),
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
			corev1.ContainerPort{
				ContainerPort: 8080,
			},
		},
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/ready",
					Port: intstr.FromInt(8080),
				},
			},
			InitialDelaySeconds: 1,
			PeriodSeconds:       3,
		},
	}

	for _, v := range extraVolumes {
		cnt.VolumeMounts = append(cnt.VolumeMounts, corev1.VolumeMount{
			MountPath: fmt.Sprintf("/%s", v.Name),
			Name:      v.Name,
			ReadOnly:  true,
		})
	}

	volumes := append(extraVolumes, corev1.Volume{
		Name: "oidc",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "oidc-ca",
			},
		},
	})

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oidc-ca",
			Namespace: ns.Name,
		},
		Data: map[string][]byte{
			"ca.pem": oidcKeyBundle.CertBytes,
		},
	}

	_, err := h.KubeClient.CoreV1().Secrets(ns.Name).Create(sec)
	if err != nil {
		return nil, nil, err
	}

	bundle, appURL, err := h.deployApp(ns.Name, ProxyName, corev1.ServiceTypeNodePort, cnt, volumes...)
	if err != nil {
		return nil, nil, err
	}

	pTrue := true
	pFalse := false

	crole, err := h.KubeClient.RbacV1().ClusterRoles().Create(&rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: ProxyName + "-",
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion:         "core/v1",
					BlockOwnerDeletion: &pTrue,
					Controller:         &pFalse,
					Kind:               "Namespace",
					Name:               ns.Name,
					UID:                ns.UID,
				},
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"users", "groups", "serviceaccounts"},
				Verbs:     []string{"impersonate"},
			},
			{
				APIGroups: []string{"authentication.k8s.io"},
				Resources: []string{"userextras/scopes", "tokenreviews"},
				Verbs:     []string{"impersonate", "create"},
			},
		},
	})
	if err != nil {
		return nil, nil, err
	}

	_, err = h.KubeClient.RbacV1().ClusterRoleBindings().Create(
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: ProxyName + "-",
				OwnerReferences: []metav1.OwnerReference{
					metav1.OwnerReference{
						APIVersion:         "core/v1",
						BlockOwnerDeletion: &pTrue,
						Controller:         &pFalse,
						Kind:               "Namespace",
						Name:               ns.Name,
						UID:                ns.UID,
					},
				},
			},
			RoleRef: rbacv1.RoleRef{
				Name: crole.Name, Kind: "ClusterRole",
			},
			Subjects: []rbacv1.Subject{
				{Name: ProxyName, Namespace: ns.Name, Kind: "ServiceAccount"},
			},
		})
	if err != nil {
		return nil, nil, err
	}

	return bundle, appURL, nil
}

func (h *Helper) DeployIssuer(ns string) (*util.KeyBundle, *url.URL, error) {
	cnt := corev1.Container{
		Name:            IssuerName,
		Image:           IssuerName,
		ImagePullPolicy: corev1.PullNever,
		Args: []string{
			"oidc-issuer",
			"--secure-port=6443",
			fmt.Sprintf("--issuer-url=https://oidc-issuer-e2e.%s.svc.cluster.local:6443", ns),
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

	bundle, appURL, err := h.deployApp(ns, IssuerName, corev1.ServiceTypeClusterIP, cnt)
	if err != nil {
		return nil, nil, err
	}

	return bundle, appURL, nil
}

func (h *Helper) DeployFakeAPIServer(ns string) ([]corev1.Volume, *url.URL, error) {
	cnt := corev1.Container{
		Name:            FakeAPIServerName,
		Image:           FakeAPIServerName,
		ImagePullPolicy: corev1.PullNever,
		Args: []string{
			"fake-apiserver",
			"--secure-port=6443",
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

	bundle, appURL, err := h.deployApp(ns, FakeAPIServerName, corev1.ServiceTypeClusterIP, cnt)
	if err != nil {
		return nil, nil, err
	}

	sec, err := h.KubeClient.CoreV1().Secrets(ns).Create(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "fake-apiserver-ca-",
			Namespace:    ns,
		},
		Data: map[string][]byte{
			"ca.pem": bundle.CertBytes,
		},
	})
	if err != nil {
		return nil, nil, err
	}

	extraVolumes := []corev1.Volume{
		{
			Name: "fake-apiserver",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: sec.Name,
				},
			},
		},
	}

	return extraVolumes, appURL, nil
}

func (h *Helper) deployApp(ns, name string, serviceType corev1.ServiceType, container corev1.Container, volumes ...corev1.Volume) (*util.KeyBundle, *url.URL, error) {
	host, appURL := h.appURL(ns, name, "6443")

	var netIPs []net.IP
	if serviceType == corev1.ServiceTypeNodePort {
		nodes, err := h.KubeClient.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			return nil, nil, err
		}

		for _, n := range nodes.Items {
			for _, addr := range n.Status.Addresses {
				if addr.Type == corev1.NodeInternalIP {
					netIPs = append(netIPs, net.ParseIP(addr.Address))
				}
			}
		}
	}

	keyBundle, err := util.NewTLSSelfSignedCertKey(host, netIPs, nil)
	if err != nil {
		return nil, nil, err
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
			Type: serviceType,
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
			"cert.pem": keyBundle.CertBytes,
			"key.pem":  keyBundle.KeyBytes,
		},
	}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				"app": name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},

				Spec: corev1.PodSpec{
					ServiceAccountName: name,
					Containers:         []corev1.Container{container},
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
			},
		},
	}

	svc, err = h.KubeClient.CoreV1().Services(ns).Create(svc)
	if err != nil {
		return nil, nil, err
	}

	if len(netIPs) > 0 {
		appURL = fmt.Sprintf("https://%s:%s", netIPs[0],
			strconv.FormatUint(uint64(svc.Spec.Ports[0].NodePort), 10))
	}

	_, err = h.KubeClient.CoreV1().Secrets(ns).Create(sec)
	if err != nil {
		return nil, nil, err
	}

	_, err = h.KubeClient.CoreV1().ServiceAccounts(ns).Create(sa)
	if err != nil {
		return nil, nil, err
	}

	_, err = h.KubeClient.AppsV1().Deployments(ns).Create(deploy)
	if err != nil {
		return nil, nil, err
	}

	if err := h.WaitForDeploymentReady(ns, name, time.Second*20); err != nil {
		return nil, nil, err
	}

	appNetURL, err := url.Parse(appURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse app url %q: %s",
			appURL, err)
	}

	return keyBundle, appNetURL, nil
}

func (h *Helper) DeleteProxy(ns string) error {
	return h.deleteApp(ns, ProxyName, "oidc-ca")
}
func (h *Helper) DeleteIssuer(ns string) error {
	return h.deleteApp(ns, IssuerName)
}
func (h *Helper) DeleteFakeAPIServer(ns string) error {
	return h.deleteApp(ns, FakeAPIServerName)
}

func (h *Helper) deleteApp(ns, name string, extraSecrets ...string) error {
	err := h.KubeClient.AppsV1().Deployments(ns).Delete(name, nil)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	for _, s := range append(extraSecrets, name) {
		err = h.KubeClient.CoreV1().Secrets(ns).Delete(s, nil)
		if err != nil && !k8sErrors.IsNotFound(err) {
			return err
		}
	}

	err = h.KubeClient.CoreV1().Services(ns).Delete(name, nil)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	err = h.KubeClient.CoreV1().ServiceAccounts(ns).Delete(name, nil)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (h *Helper) appURL(ns, serviceName, port string) (string, string) {
	host := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, ns)
	return host, fmt.Sprintf("https://%s:%s", host, port)
}
