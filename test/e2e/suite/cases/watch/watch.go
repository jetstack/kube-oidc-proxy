// Copyright Jetstack Ltd. See LICENSE for details.
package passthrough

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jetstack/kube-oidc-proxy/test/e2e/framework"
	"github.com/jetstack/kube-oidc-proxy/test/e2e/framework/helper"
)

var _ = framework.CasesDescribe("Watch", func() {
	f := framework.NewDefaultFramework("watch")

	It("pod should restart if a mounted ConfigMap that is watched updates its contents", func() {
		By("Creating ConfigMap that will be mounted into pod")
		cm, err := f.Helper().KubeClient.CoreV1().ConfigMaps(f.Namespace.Name).Create(&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "kube-oidc-proxy-e2e-watch-",
				Namespace:    f.Namespace.Name,
			},
			Data: map[string]string{
				"key-1": "this is some data",
				"key-2": "this is some more data",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		By("ReDeploying Proxy with watched ConfigMap")

		f.DeployProxyWith([]string{
			"--reload-watch-refresh-period=5s",
			"--reload-watch-files=/configmap/key-1,/configmap/key-2",
		},
			helper.AddProxyVolumeMounts([]corev1.VolumeMount{
				corev1.VolumeMount{
					MountPath: "/configmap",
					Name:      "configmap",
					ReadOnly:  true,
				},
			}),
			helper.AddProxyVolumes([]corev1.Volume{
				corev1.Volume{
					Name: "configmap",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: cm.Name,
							},
							Items: []corev1.KeyToPath{
								{Key: "key-1", Path: "key-1"},
								{Key: "key-2", Path: "key-2"},
							},
						},
					},
				},
			}),
		)

		By("Update ConfigMap Data")
		cm.Data["key-1"] = "this is different data"
		cm.Data["key-2"] = "this is more different data"
		_, err = f.Helper().KubeClient.CoreV1().ConfigMaps(f.Namespace.Name).Update(cm)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for Proxy to Stop")
		err = f.Helper().WaitForPodNotReady(f.Namespace.Name, helper.ProxyName, time.Second*20)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for Proxy to Become Ready Again")
		err = f.Helper().WaitForPodReady(f.Namespace.Name, helper.ProxyName, time.Second*20)
		Expect(err).NotTo(HaveOccurred())
	})

	It("pod should restart if a mounted Secret that is watched updates its contents", func() {
		By("Creating Secret that will be mounted into pod")
		sec, err := f.Helper().KubeClient.CoreV1().Secrets(f.Namespace.Name).Create(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "kube-oidc-proxy-e2e-watch-",
				Namespace:    f.Namespace.Name,
			},
			Data: map[string][]byte{
				"key-1": []byte("this is some data"),
				"key-2": []byte("this is some more data"),
			},
		})
		Expect(err).NotTo(HaveOccurred())

		By("ReDeploying Proxy with watched Secret")

		f.DeployProxyWith([]string{
			"--reload-watch-refresh-period=5s",
			"--reload-watch-files=/configmap/key-1,/configmap/key-2",
		},
			helper.AddProxyVolumeMounts([]corev1.VolumeMount{
				corev1.VolumeMount{
					MountPath: "/configmap",
					Name:      "configmap",
					ReadOnly:  true,
				},
			}),
			helper.AddProxyVolumes([]corev1.Volume{
				corev1.Volume{
					Name: "configmap",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: sec.Name,
							Items: []corev1.KeyToPath{
								{Key: "key-1", Path: "key-1"},
								{Key: "key-2", Path: "key-2"},
							},
						},
					},
				},
			}),
		)

		By("Update Secret Data")
		sec.Data["key-1"] = []byte("this is different data")
		sec.Data["key-2"] = []byte("this is more different data")
		_, err = f.Helper().KubeClient.CoreV1().Secrets(f.Namespace.Name).Update(sec)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for Proxy to Stop")
		err = f.Helper().WaitForPodNotReady(f.Namespace.Name, helper.ProxyName, time.Second*20)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for Proxy to Become Ready Again")
		err = f.Helper().WaitForPodReady(f.Namespace.Name, helper.ProxyName, time.Second*20)
		Expect(err).NotTo(HaveOccurred())
	})
})
