# Reload on File Changes

When rotating certificates, kube-oidc-proxy needs to be restarted in order to
begin serving using its new certificate. kube-oidc-proxy can be configured to
watch for local files that, when are changed, will send itself a SIGHUP signal.
Once received, no new connections can be established and a graceful shutdown of
current connections will take place. This behaviour is based on periodic polling
of target file mod times.

To enable files for watching, append a comma separated list of file paths to the
following flag:

```
--reload-watch-files=/etc/oidc/tls/crt.pem,/etc/oidc/tls/key.pem
```

The polling period can be configured to be of 1 second or more. The flag expects
a duration string as defined by the [Go
Documentation](https://golang.org/pkg/time/#ParseDuration).

```
--reload-watch-refresh-period=10s
```
