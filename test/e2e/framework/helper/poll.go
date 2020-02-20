// Copyright Jetstack Ltd. See LICENSE for details.
package helper

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (h *Helper) WaitForDeploymentReady(namespace, name string, timeout time.Duration) error {
	log.Infof("Waiting for Deployment to become ready %s/%s", namespace, name)

	err := wait.PollImmediate(time.Second*2, timeout, func() (bool, error) {
		deploy, err := h.KubeClient.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if deploy.Spec.Replicas == nil {
			return false, nil
		}

		if *deploy.Spec.Replicas == deploy.Status.ReadyReplicas {
			return true, nil
		}

		return false, nil
	})

	if err != nil {
		kErr := h.Kubectl(namespace).DescribeResource("deployment", name)
		if kErr != nil {
			err = fmt.Errorf("%s\n%s", err, kErr)
		}

		return err
	}

	return nil
}

func (h *Helper) WaitForPodReady(namespace, name string, timeout time.Duration) error {
	log.Infof("Waiting for Pod to become ready %s/%s", namespace, name)

	err := wait.PollImmediate(time.Second*2, timeout, func() (bool, error) {
		pod, err := h.KubeClient.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if len(pod.Status.Conditions) == 0 {
			return false, nil
		}

		var ready bool
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady &&
				cond.Status == corev1.ConditionTrue {
				ready = true
				break
			}
		}

		if !ready {
			log.Infof("helper: pod not ready %s/%s: %v",
				pod.Namespace, pod.Name, pod.Status.Conditions)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		kErr := h.Kubectl(namespace).DescribeResource("pod", name)
		if kErr != nil {
			err = fmt.Errorf("%s\n%s", err, kErr)
		}

		return err
	}

	return nil
}

func (h *Helper) WaitForDeploymentToDelete(namespace, name string, timeout time.Duration) error {
	log.Infof("Waiting for Deployment to be deleted: %s/%s", namespace, name)

	err := wait.PollImmediate(time.Second*2, timeout, func() (bool, error) {
		_, err := h.KubeClient.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
		if k8sErrors.IsNotFound(err) {
			return true, nil
		}

		if err != nil {
			return false, nil
		}

		return false, nil
	})

	if err != nil {
		kErr := h.Kubectl(namespace).DescribeResource("deployment", name)
		if kErr != nil {
			err = fmt.Errorf("%s\n%s", err, kErr)
		}

		return err
	}

	return nil
}
