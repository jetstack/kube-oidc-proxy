# No Impersonation

kube-oidc-proxy can be configured to disable impersonation. When a request has
been successfully authenticated, the request is forwarded as-is, without changes
to the HTTP header and no authentication injected by the proxy. The OIDC
bearer token is also kept in the request. This can be useful for securing
endpoints that do not provide OIDC or any authentication methods and do not
implement any authorization.

To disable impersonation, provide the following flag:

```
--disable-impersonation
```
