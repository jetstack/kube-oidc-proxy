package helper

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"time"

	jose "gopkg.in/square/go-jose.v2"
	"k8s.io/client-go/rest"

	"github.com/jetstack/kube-oidc-proxy/test/e2e/util"
)

func (h *Helper) NewValidRestConfig(issuerBundle, proxyBundle *util.KeyBundle, issuerURL, proxyURL, clientID string) (*rest.Config, error) {
	signer, err := jose.NewSigner(jose.SigningKey{
		Algorithm: jose.SignatureAlgorithm("RS256"),
		Key:       issuerBundle.Key,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise new jwt signer: %s", err)
	}

	token := h.newValidToken(issuerURL, clientID)

	jwt, err := signer.Sign(token)
	if err != nil {
		return nil, fmt.Errorf("failed to sign jwt: %s", err)
	}

	signedToken, err := jwt.CompactSerialize()
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(proxyBundle.CertBytes); !ok {
		return nil, fmt.Errorf("failed to append proxy cert data to cert pool %s", proxyBundle.CertBytes)
	}

	return &rest.Config{
		Host:        proxyURL,
		Burst:       rest.DefaultBurst,
		BearerToken: signedToken,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPool,
			},
		},
	}, nil
}

func (h *Helper) newValidToken(issuerURL, clientID string) []byte {
	// valid for 10 mins
	return []byte(fmt.Sprintf(`{
	"iss":"%s",
	"aud":["%s","aud-2"],
	"email":"foo@example.com",
	"groups":["group-1","group-2"],
	"exp":%d
	}`, issuerURL, clientID, time.Now().Add(time.Minute*10).Unix()))
}
