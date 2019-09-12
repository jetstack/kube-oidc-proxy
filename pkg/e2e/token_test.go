// Copyright Jetstack Ltd. See LICENSE for details.
package e2e

import (
	"fmt"
	"testing"
	"time"
)

const (
	namespaceTokenTest = "kube-oidc-proxy-e2e-token"
)

func Test_Token(t *testing.T) {
	mustSkipMissingSuite(t)
	mustNamespace(t, namespaceTokenTest)
	mustCreatePodRbac(t, "test-username", namespaceTokenTest, "User")

	url := fmt.Sprintf(
		"https://127.0.0.1:%s/api/v1/namespaces/%s/pods",
		e2eSuite.proxyPort,
		namespaceTokenTest,
	)

	// valid token
	e2eSuite.testToken(
		t,
		e2eSuite.validToken(),
		url,
		200,
		"")

	for _, test := range []struct {
		token   []byte
		expCode int
		expBody string
	}{

		// no bearer token
		{
			token:   nil,
			expCode: 401,
			expBody: "Unauthorized",
		},

		//	 invalid bearer token
		{
			token:   []byte("bad-payload"),
			expCode: 401,
			expBody: "Unauthorized",
		},

		// wrong issuer
		{
			token: []byte(fmt.Sprintf(`{
	"iss":"incorrect-issuer",
	"aud":["kube-oidc-proxy_e2e_client-id","aud-2"],
	"e2e-username-claim":"test-username",
	"e2e-groups-claim":["group-1","group-2"],
	"exp":%d
	}`, time.Now().Add(time.Minute).Unix())),
			expCode: 401,
			expBody: "Unauthorized",
		},

		// no audience
		{
			token: []byte(fmt.Sprintf(`{
	"iss":"https://127.0.0.1:%s",
	"aud":[],
	"e2e-username-claim":"test-username",
	"e2e-groups-claim":["group-1","group-2"],
	"exp":%d
	}`, e2eSuite.issuer.Port(), time.Now().Add(time.Minute).Unix())),
			expCode: 401,
			expBody: "Unauthorized",
		},

		// wrong audience
		{
			token: []byte(fmt.Sprintf(`{
	"iss":"https://127.0.0.1:%s",
	"aud":["aud-1", "aud-2"],
	"e2e-username-claim":"test-username",
	"e2e-groups-claim":["group-1","group-2"],
	"exp":%d
	}`, e2eSuite.issuer.Port(), time.Now().Add(time.Minute).Unix())),
			expCode: 401,
			expBody: "Unauthorized",
		},

		// token expires now
		{
			token: []byte(fmt.Sprintf(`{
	"iss":"https://127.0.0.1:%s",
	"aud":["kube-oidc-proxy_e2e_client-id","aud-2"],
	"e2e-username-claim":"test-username",
	"e2e-groups-claim":["group-1","group-2"],
	"exp":%d
	}`, e2eSuite.issuer.Port(), time.Now().Unix())),
			expCode: 401,
			expBody: "Unauthorized",
		},
	} {
		e2eSuite.testToken(t, test.token, url, test.expCode, test.expBody)
	}
}
