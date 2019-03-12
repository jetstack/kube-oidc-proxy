// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import "k8s.io/client-go/rest"

type E2ESuite struct {
	apiserverCnf *rest.Config
	proxyCnf     *rest.Config
}

func New(apiserverCnf, proxyCnf *rest.Config) *E2ESuite {
	return &E2ESuite{
		apiserverCnf: apiserverCnf,
		proxyCnf:     proxyCnf,
	}
}
