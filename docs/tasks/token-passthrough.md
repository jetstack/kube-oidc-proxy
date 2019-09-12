# Token Passthrough

kube-oidc-proxy can be configured to enable 'token passthrough' for tokens that
fail OIDC authentication. If enabled, kube-oidc-proxy will perform a [token
review](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#webhook-token-authentication)
API call to the configured target backend using the Kubernetes API. If
successful, the request will be passed through as-is, with the token intact in
the request and no other authentication used by kube-oidc-proxy.

To enable token passthrough, include the following flag:

```
--token-passthrough
```

In the case of the Kubernetes API server, the authenticator, if audience aware,
will validate the audiences of tokens using the audience of the API server. A
new set of audiences can also be given which will be used to validate the token
against. At least one of these audiences need to be present in the audiences of
the token to be successful:

```
---token-passthrough-audiences=aud1.foo.bar,aud2.foo.bar
```
