// Copyright Jetstack Ltd. See LICENSE for details.
package options

import (
	"net"

	"github.com/spf13/pflag"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	cliflag "k8s.io/component-base/cli/flag"
)

type SecureServingOptions struct {
	*apiserveroptions.SecureServingOptions
}

func NewSecureServingOptions(nfs *cliflag.NamedFlagSets) *SecureServingOptions {
	s := &SecureServingOptions{
		SecureServingOptions: &apiserveroptions.SecureServingOptions{
			BindAddress: net.ParseIP("0.0.0.0"),
			BindPort:    6443,
			Required:    true,
			ServerCert: apiserveroptions.GeneratableKeyCert{
				PairName:      AppName,
				CertDirectory: "/var/run/kubernetes",
			},
		},
	}

	return s.AddFlags(nfs.FlagSet("Secure Serving"))
}

func (s *SecureServingOptions) AddFlags(fs *pflag.FlagSet) *SecureServingOptions {
	s.SecureServingOptions.AddFlags(fs)
	return s
}
