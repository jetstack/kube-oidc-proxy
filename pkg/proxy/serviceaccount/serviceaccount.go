// Copyright Jetstack Ltd. See LICENSE for details.
package serviceaccount

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	clientgoinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/keyutil"
	"k8s.io/klog"

	"github.com/jetstack/kube-oidc-proxy/cmd/options"
	serviceaccount "github.com/jetstack/kube-oidc-proxy/pkg/proxy/serviceaccount/authenticator"
	serviceaccountgetter "github.com/jetstack/kube-oidc-proxy/pkg/proxy/serviceaccount/getter"
)

var (
	ErrUnAuthed   = errors.New("service account token authentication failed")
	ErrTokenParse = errors.New("no bearer token found in request header")
)

type Authenticator struct {
	scopedAuther authenticator.Token
	legacyAuther authenticator.Token

	lookup bool
}

// Returns a new validator to validate service account tokens. Supports both
// legacy and scoped tokens. If lookup is disabled, scoped tokens cannot be
// checked so are also disabled.
func New(restConfig *rest.Config,
	options *options.ServiceAccountAuthenticationOptions,
	apiAudiences []string) (*Authenticator, error) {
	allPublicKeys := []interface{}{}
	for _, keyfile := range options.KeyFiles {
		publicKeys, err := keyutil.PublicKeysFromFile(keyfile)
		if err != nil {
			return nil, err
		}

		allPublicKeys = append(allPublicKeys, publicKeys...)
	}

	var scopedAuther, legacyAuther authenticator.Token
	var getter serviceaccount.ServiceAccountTokenGetter

	// Only build scoped token validator and init getter if we have lookup enabled.
	// Scoped token validator requires API lookups so this also disables it.
	if options.Lookup {
		client, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return nil, err
		}

		informer := clientgoinformers.NewSharedInformerFactory(client, 10*time.Minute)

		getter = serviceaccountgetter.NewGetterFromClient(
			client,
			informer.Core().V1().Secrets().Lister(),
			informer.Core().V1().ServiceAccounts().Lister(),
			informer.Core().V1().Pods().Lister(),
		)

		scopedValidator := serviceaccount.NewValidator(getter)
		scopedAuther = serviceaccount.JWTTokenAuthenticator(options.Issuer, allPublicKeys, apiAudiences, scopedValidator)
	}

	legacyValidator := serviceaccount.NewLegacyValidator(options.Lookup, getter)
	legacyAuther = serviceaccount.JWTTokenAuthenticator(options.Issuer, allPublicKeys, apiAudiences, legacyValidator)

	return &Authenticator{
		legacyAuther: legacyAuther,
		scopedAuther: scopedAuther,
		lookup:       options.Lookup,
	}, nil
}

func (a *Authenticator) Request(req *http.Request) error {
	token, err := parseTokenFromHeader(req)
	if err != nil {
		return err
	}

	if a.lookup {
		_, b, err := a.scopedAuther.AuthenticateToken(req.Context(), token)

		if err == nil && b {
			klog.Infof("token authenticated as scoped token %s", req.RemoteAddr)
			return nil
		}

		if err != nil {
			klog.Errorf("failed to authenticate request as scoped token: %s", err)
		}
	}

	_, ok, err := a.legacyAuther.AuthenticateToken(req.Context(), token)
	if err != nil {
		klog.Errorf("failed to authenticate request as legacy token: %s", err)
		return ErrUnAuthed
	}

	if !ok {
		return ErrUnAuthed
	}

	klog.Infof("token authenticated as legacy token %s", req.RemoteAddr)

	return nil
}

// Return just the token from the header of the request, without 'bearer'.
func parseTokenFromHeader(req *http.Request) (string, error) {
	if req == nil || req.Header == nil {
		return "", ErrTokenParse
	}

	auth := strings.TrimSpace(req.Header.Get("Authorization"))
	if auth == "" {
		return "", ErrTokenParse
	}

	parts := strings.Split(auth, " ")
	if len(parts) < 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", ErrTokenParse
	}

	token := parts[1]

	// Empty bearer tokens aren't valid
	if len(token) == 0 {
		return "", ErrTokenParse
	}

	return token, nil
}
