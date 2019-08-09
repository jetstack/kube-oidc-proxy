// +build tools

// Copyright Jetstack Ltd. See LICENSE for details.
package tools

// This file is used to vendor packages we use to build binaries.
// This is the current canonical way with go modules.

import (
	_ "github.com/golang/mock/mockgen"
)
