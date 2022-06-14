// Copyright Jetstack Ltd. See LICENSE for details.
package helper

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
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
		deploy, err := h.KubeClient.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
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
		pod, err := h.KubeClient.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
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
		_, err := h.KubeClient.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if k8sErrors.IsNotFound(err) {
			log.Infof("Deployment %s/%s deleted, waiting for pods", namespace, name)
			pods, err := h.KubeClient.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})

			if err != nil {
				return false, nil
			}

			foundPods := false

			for _, pod := range pods.Items {
				if strings.HasPrefix(pod.ObjectMeta.Name, name+"-") {
					log.Infof("Pod %s/%s still not terminated", namespace, &pod.ObjectMeta.Name)
					foundPods = true
					return false, nil
				}
			}

			log.Infof("All pods for %s/%s terminated", namespace, name)
			return !foundPods, nil
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

func (h *Helper) WaitForUrlToBeReady(url *url.URL, timeout time.Duration) error {
	log.Infof("Waiting for URL %s to be ready", url)

	err := wait.PollImmediate(time.Second*2, timeout, func() (bool, error) {
		host := url.Host
		port := url.Port()
		tocheck := host

		if port != "" {
			tocheck = tocheck + ":" + port
		}

		con, err := net.DialTimeout("tcp", tocheck, timeout)
		if err != nil {
			return false, nil
		} else {
			con.Close()
			return true, nil
		}

	})

	return err
}
