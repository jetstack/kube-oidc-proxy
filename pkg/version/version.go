// Copyright Jetstack Ltd. See LICENSE for details.
package version

import (
	"fmt"
	"os"
	"runtime"

	apimachineryversion "k8s.io/apimachinery/pkg/version"
)

const (
	appName = "kube-oidc-proxy"
)

var (
	gitMajor     string // major version, always numeric
	gitMinor     string // minor version, numeric possibly followed by "+"
	gitVersion   = "v0.0.0-master+$Format:%h$"
	gitCommit    = "$Format:%H$" // sha1 from git, output of $(git rev-parse HEAD)
	gitTreeState = ""            // state of git tree, either "clean" or "dirty"

	buildDate = "1970-01-01T00:00:00Z" // build date in ISO8601 format, output of $(date -u +'%Y-%m-%dT%H:%M:%SZ')
)

func PrintVersionAndExit() {
	fmt.Printf("%s version: %#v\n", appName,
		apimachineryversion.Info{
			Major:        gitMajor,
			Minor:        gitMinor,
			GitVersion:   gitVersion,
			GitCommit:    gitCommit,
			GitTreeState: gitTreeState,
			BuildDate:    buildDate,
			GoVersion:    runtime.Version(),
			Compiler:     runtime.Compiler,
			Platform:     fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		},
	)

	os.Exit(0)
}
