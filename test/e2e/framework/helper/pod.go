package helper

import (
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (h *Helper) WaitForPodReady(namespace, name string, timeout time.Duration) error {
	log.Infof("Waiting for Pod to become ready %s/%s", namespace, name)

	err := wait.PollImmediate(time.Second*5, timeout, func() (bool, error) {
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
		h.Kubectl(namespace).DescribeResource("pod", name)
		return err
	}

	return nil
}

func (h *Helper) WaitForPodDeletion(namespace, name string, timeout time.Duration) error {
	log.Infof("Waiting for Pod to be deleted %s/%s", namespace, name)

	err := wait.PollImmediate(time.Second*5, timeout, func() (bool, error) {
		pod, err := h.KubeClient.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
		if k8sErrors.IsNotFound(err) {
			return true, nil
		}

		if err != nil {
			return false, err
		}

		log.Infof("helper: pod not deleted %s/%s: %v",
			pod.Namespace, pod.Name, pod.Status.Conditions)

		return false, nil
	})
	if err != nil {
		h.Kubectl(namespace).DescribeResource("pod", name)
		return err
	}

	return nil
}
