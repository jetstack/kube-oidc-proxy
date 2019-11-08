// Copyright Jetstack Ltd. See LICENSE for details.
package framework

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"

	"github.com/jetstack/kube-oidc-proxy/test/e2e/framework/config"
	"github.com/jetstack/kube-oidc-proxy/test/e2e/framework/helper"
)

type Framework struct {
	BaseName string

	KubeClientSet kubernetes.Interface

	Namespace *corev1.Namespace

	config *config.Config
	helper *helper.Helper
}

func NewFramework(baseName string, config *config.Config) *Framework {
	f := &Framework{
		BaseName: baseName,
		config:   config,
	}

	JustBeforeEach(f.BeforeEach)
	AfterEach(f.AfterEach)

	return f
}

func (f *Framework) BeforeEach() {
	f.helper = helper.NewHelper(f.config)

	By("Creating a kubernetes client")

	clientConfigFlags := genericclioptions.NewConfigFlags(true)
	clientConfigFlags.KubeConfig = &f.config.KubeConfigPath
	config, err := clientConfigFlags.ToRESTConfig()
	Expect(err).NotTo(HaveOccurred())

	f.KubeClientSet, err = kubernetes.NewForConfig(config)
	Expect(err).NotTo(HaveOccurred())

	By("Building a namespace api object")
	f.Namespace, err = f.CreateKubeNamespace(f.BaseName)
	Expect(err).NotTo(HaveOccurred())

	By("Using the namespace " + f.Namespace.Name)

	f.helper.KubeClient = f.KubeClientSet
}

// AfterEach deletes the namespace, after reading its events.
func (f *Framework) AfterEach() {
	By("Deleting test namespace")
	err := f.DeleteKubeNamespace(f.Namespace.Name)
	Expect(err).NotTo(HaveOccurred())

	By("Waiting for test namespace to no longer exist")
	err = f.WaitForKubeNamespaceNotExist(f.Namespace.Name)
	Expect(err).NotTo(HaveOccurred())
}

func (f *Framework) Helper() *helper.Helper {
	return f.helper
}
