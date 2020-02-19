// Copyright Jetstack Ltd. See LICENSE for details.
package options

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/pflag"
	apimachineryversion "k8s.io/apimachinery/pkg/version"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/cli/globalflag"
)

type MiscOptions struct {
	gitMajor     string // major version, always numeric
	gitMinor     string // minor version, numeric possibly followed by "+"
	gitVersion   string
	gitCommit    string // sha1 from git, output of $(git rev-parse HEAD)
	gitTreeState string // state of git tree, either "clean" or "dirty"

	buildDate string // build date in ISO8601 format, output of $(date -u +'%Y-%m-%dT%H:%M:%SZ')
}

var (
	gitMajor     string // major version, always numeric
	gitMinor     string // minor version, numeric possibly followed by "+"
	gitVersion   = "v0.0.0-master+$Format:%h$"
	gitCommit    = "$Format:%H$" // sha1 from git, output of $(git rev-parse HEAD)
	gitTreeState = ""            // state of git tree, either "clean" or "dirty"

	buildDate = "1970-01-01T00:00:00Z" // build date in ISO8601 format, output of $(date -u +'%Y-%m-%dT%H:%M:%SZ')
)

func NewMiscOptions(nfs *cliflag.NamedFlagSets) *MiscOptions {
	m := &MiscOptions{
		gitMajor:     gitMajor,
		gitMinor:     gitMinor,
		gitVersion:   gitVersion,
		gitCommit:    gitCommit,
		gitTreeState: gitTreeState,
		buildDate:    buildDate,
	}

	return m.AddFlags(nfs.FlagSet("Misc"))
}

func (m *MiscOptions) AddFlags(fs *pflag.FlagSet) *MiscOptions {
	globalflag.AddGlobalFlags(fs, AppName)
	fs.Bool("version", false, "Print version information and quit")
	return m
}

func (m *MiscOptions) PrintVersionAndExit() {
	fmt.Printf("%s version: %#v\n", AppName,
		apimachineryversion.Info{
			Major:        m.gitMajor,
			Minor:        m.gitMinor,
			GitVersion:   m.gitVersion,
			GitCommit:    m.gitCommit,
			GitTreeState: m.gitTreeState,
			BuildDate:    m.buildDate,
			GoVersion:    runtime.Version(),
			Compiler:     runtime.Compiler,
			Platform:     fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		},
	)

	os.Exit(0)
}
