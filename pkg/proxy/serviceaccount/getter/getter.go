// Copyright Jetstack Ltd. See LICENSE for details.
package getter

import (
	"k8s.io/api/core/v1"
)

type FakeGetter struct{}

func (f *FakeGetter) GetServiceAccount(namespace, name string) (*v1.ServiceAccount, error) {
	return nil, nil
}

func (f *FakeGetter) GetPod(namespace, name string) (*v1.Pod, error) {
	return nil, nil
}

func (f *FakeGetter) GetSecret(namespace, name string) (*v1.Secret, error) {
	return nil, nil
}
