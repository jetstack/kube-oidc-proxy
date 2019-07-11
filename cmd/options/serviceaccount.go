// Copyright Jetstack Ltd. See LICENSE for details.
package options

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/pflag"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type ServiceAccountAuthenticationOptions struct {
	KeyFiles      []string
	Lookup        bool
	Issuer        string
	MaxExpiration time.Duration
}

func (s *ServiceAccountAuthenticationOptions) Validate() error {
	var errs []error

	if len(s.Issuer) > 0 && strings.Contains(s.Issuer, ":") {
		if _, err := url.Parse(s.Issuer); err != nil {
			errs = append(errs, fmt.Errorf("service-account-issuer contained a ':' but was not a valid URL: %v", err))
		}
	}
	return utilerrors.NewAggregate(errs)
}

func (s *ServiceAccountAuthenticationOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringArrayVar(&s.KeyFiles, "service-account-key-file", s.KeyFiles, ""+
		"File containing PEM-encoded x509 RSA or ECDSA private or public keys, used to verify "+
		"ServiceAccount tokens. The specified file can contain multiple keys, and the flag can "+
		"be specified multiple times with different files. If unspecified, "+
		"--tls-private-key-file is used. Must be specified when "+
		"--service-account-signing-key is provided")

	fs.BoolVar(&s.Lookup, "service-account-lookup", s.Lookup,
		"If true, validate ServiceAccount tokens exist in etcd as part of authentication.")

	fs.StringVar(&s.Issuer, "service-account-issuer", s.Issuer, ""+
		"Identifier of the service account token issuer. The issuer will assert this identifier "+
		"in \"iss\" claim of issued tokens. This value is a string or URI.")

	fs.DurationVar(&s.MaxExpiration, "service-account-max-token-expiration", s.MaxExpiration, ""+
		"The maximum validity duration of a token created by the service account token issuer. If an otherwise valid "+
		"TokenRequest with a validity duration larger than this value is requested, a token will be issued with a validity duration of this value.")
}
