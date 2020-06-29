# Metrics

kube-oidc-proxy exposes a number of Prometheus metrics to give some insights
into how the proxy is performing. These metrics are designed to be used to
better inform how the proxy is behaving, and the general usage from clients,
*_not_* to alert, or otherwise give other security insights. If you are
interested in auditing and access review functionality, please refer to
[auditing](../auditing.md).

The proxy exposes the following metrics:

### kube_oidc_proxy_http_client_requests
counter - {http status code, path, remote address}
The number of incoming requests.

### kube_oidc_proxy_http_client_duration_seconds
histogram - {remote address}
The duration in seconds for incoming client requests to be responded to.

### kube_oidc_proxy_http_server_requests
counter - {http status code, path, remote address}
The number of outgoing server requests.

### kube_oidc_proxy_http_server_duration_seconds
histogram - {remote address}
The duration in seconds for outgoing server requests to be responded to.

### kube_oidc_proxy_token_review_duration_seconds
histogram - {authenticated, http status code, remote address, user}
The duration in seconds for a token review lookup. Authenticated requests are 1, else 0.

### kube_oidc_proxy_oidc_authentication_count
counter - {authenticated, remote address, user}
The count for OIDC authentication. Authenticated requests are 1, else 0.

## Metrics Address

By default, metrics are exposed on `0.0.0.0:80/metrics`. The flag
`--metrics-serving-address` can be used to change the address, however the
`/metrics` path will remain the same. The metrics address must _not_ conflict
with the proxy and probe addresses. Setting `--metrics-serving-address=""` will
disable the metrics server.
