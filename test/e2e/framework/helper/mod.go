// Copyright Jetstack Ltd. See LICENSE for details.
package helper

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/jetstack/kube-oidc-proxy/test/e2e/util"
)

type KubeOIDCProxyDeploy struct {
	h *Helper

	volumes []corev1.Volume
	*corev1.Container
}

type KubeOIDCProxyModifier func(*KubeOIDCProxyDeploy)

func (h *Helper) ReDeployProxy(ns *corev1.Namespace, issuerURL, clientID string,
	oidcKeyBundle *util.KeyBundle, extraArgs []string, mods ...KubeOIDCProxyModifier) (*util.KeyBundle, string, error) {
	k := &KubeOIDCProxyDeploy{
		h:         h,
		Container: h.proxyContainer(issuerURL, clientID, extraArgs...),
	}

	for _, mod := range mods {
		mod(k)
	}

	return h.deployProxy(ns, issuerURL, clientID, oidcKeyBundle, k.Container, k.volumes)
}

func AddProxyVolumes(vols []corev1.Volume) KubeOIDCProxyModifier {
	return func(k *KubeOIDCProxyDeploy) {
		k.volumes = append(k.volumes, vols...)
	}
}

func AddProxyVolumeMounts(mounts []corev1.VolumeMount) KubeOIDCProxyModifier {
	return func(k *KubeOIDCProxyDeploy) {
		k.Container.VolumeMounts = append(k.Container.VolumeMounts, mounts...)
	}
}
