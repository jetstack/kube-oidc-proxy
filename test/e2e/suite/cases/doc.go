// Copyright Jetstack Ltd. See LICENSE for details.
package cases

import (
	_ "github.com/jetstack/kube-oidc-proxy/test/e2e/suite/cases/impersonation"
	_ "github.com/jetstack/kube-oidc-proxy/test/e2e/suite/cases/passthrough"
	_ "github.com/jetstack/kube-oidc-proxy/test/e2e/suite/cases/probe"
	_ "github.com/jetstack/kube-oidc-proxy/test/e2e/suite/cases/rbac"
	_ "github.com/jetstack/kube-oidc-proxy/test/e2e/suite/cases/token"
	_ "github.com/jetstack/kube-oidc-proxy/test/e2e/suite/cases/upgrade"
)
