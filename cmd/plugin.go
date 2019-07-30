// Copyright Jetstack Ltd. See LICENSE for details.
package cmd

import (
	// This package is required to be imported to register all client
	// plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)
