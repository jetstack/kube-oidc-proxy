// Copyright Jetstack Ltd. See LICENSE for details.
package options

import (
	"fmt"

	"github.com/spf13/pflag"

	cliflag "k8s.io/component-base/cli/flag"
)

type OIDCAuthenticationOptions struct {
	APIAudiences   []string
	CAFile         string
	ClientID       string
	IssuerURL      string
	UsernameClaim  string
	UsernamePrefix string
	GroupsClaim    string
	GroupsPrefix   string
	SigningAlgs    []string
	RequiredClaims map[string]string
}

func NewOIDCAuthenticationOptions(nfs *cliflag.NamedFlagSets) *OIDCAuthenticationOptions {
	return new(OIDCAuthenticationOptions).AddFlags(nfs.FlagSet("OIDC"))
}

func (o *OIDCAuthenticationOptions) Validate() error {
	if o != nil && (len(o.IssuerURL) > 0) != (len(o.ClientID) > 0) {
		return fmt.Errorf("oidc-issuer-url and oidc-client-id should be specified together")
	}

	return nil
}

func (o *OIDCAuthenticationOptions) AddFlags(fs *pflag.FlagSet) *OIDCAuthenticationOptions {
	fs.StringSliceVar(&o.APIAudiences, "api-audiences", o.APIAudiences, ""+
		"Identifiers of the API. This can be used as an additional list of "+
		"identifiers that exist in the target audiences of requests when "+
		"authenticating with OIDC.")

	fs.StringVar(&o.IssuerURL, "oidc-issuer-url", o.IssuerURL, ""+
		"The URL of the OpenID issuer, only HTTPS scheme will be accepted.")

	fs.StringVar(&o.ClientID, "oidc-client-id", o.ClientID,
		"The client ID for the OpenID Connect client.")

	fs.StringVar(&o.CAFile, "oidc-ca-file", o.CAFile, ""+
		"The OpenID server's certificate will be verified by one of the authorities "+
		"in the oidc-ca-file, otherwise the host's root CA set will be used")

	fs.StringVar(&o.UsernameClaim, "oidc-username-claim", "sub", ""+
		"The OpenID claim to use as the username. Note that claims other than the default ('sub') "+
		"is not guaranteed to be unique and immutable")

	fs.StringVar(&o.UsernamePrefix, "oidc-username-prefix", "", ""+
		"If provided, all usernames will be prefixed with this value. If not provided, "+
		"username claims other than 'email' are prefixed by the issuer URL to avoid "+
		"clashes. To skip any prefixing, provide the value '-'.")

	fs.StringVar(&o.GroupsClaim, "oidc-groups-claim", "", ""+
		"If provided, the name of a custom OpenID Connect claim for specifying user groups. "+
		"The claim value is expected to be a string or array of strings.")

	fs.StringVar(&o.GroupsPrefix, "oidc-groups-prefix", "", ""+
		"If provided, all groups will be prefixed with this value to prevent conflicts with "+
		"other authentication strategies.")

	fs.StringSliceVar(&o.SigningAlgs, "oidc-signing-algs", []string{"RS256"}, ""+
		"Comma-separated list of allowed JOSE asymmetric signing algorithms. JWTs with a "+
		"'alg' header value not in this list will be rejected. "+
		"Values are defined by RFC 7518 https://tools.ietf.org/html/rfc7518#section-3.1.")

	fs.Var(cliflag.NewMapStringStringNoSplit(&o.RequiredClaims), "oidc-required-claim", ""+
		"A key=value pair that describes a required claim in the ID Token. "+
		"If set, the claim is verified to be present in the ID Token with a matching value. "+
		"Repeat this flag to specify multiple claims.")

	return o
}
