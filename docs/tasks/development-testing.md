# Development Testing

In order to help development for the proxy, there are a few tools in place for
quick testing.

# Creating a Cluster

Use `make dev_cluster_create` to spin up a kind cluster locally. This will also
build the proxy and other tooling from source, build their images, and load them
onto each node.

# Deploying the Proxy

This will build the proxy and other tooling from source,build the images, and
load them onto each node. This will then deploy the proxy alongside a fake OIDC
issuer so that the proxy is fully functional. The proxy will then be reachable
from a node port service in the cluster.


```bash
make dev_cluster_deploy
```

This command will output a signed OIDC token that is valid for the proxy. You
can then make calls to the proxy, like the following:

```bash
curl -k https://172.17.0.2:30226 -H 'Authorization: bearer eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9.ewoJImlzcyI6Imh0dHBzOi8vb2lkYy1pc3N1ZXItZTJlLmt1YmUtb2lkYy1wcm94eS1lMmUtNmhiNGcuc3ZjLmNsdXN0ZXIubG9jYWw6NjQ0MyIsCgkiYXVkIjpbImt1YmUtb2lkYy1wcm94eS1lMmUtY2xpZW50LWlkIiwiYXVkLTIiXSwKCSJlbWFpbCI6InVzZXJAZXhhbXBsZS5jb20iLAoJImdyb3VwcyI6WyJncm91cC0xIiwiZ3JvdXAtMiJdLAoJImV4cCI6MTU4MjU1NTYzMQoJfQ.qWCM5zUHGslmwbgyZnMjhVeCLJd3R3c7xjtatjT_pv1VY-PpJ8IGBsbcCpur1fAm2CAbr0juM3yzwV1S3TUjhNhE8Wo6rxjA2Flnmwj7Nn2Got6T_cMFHQ_3A6YC72qkMwH-7SvXFB-C5Bk96vi9-clrxJ_b1XjfMPViZEVCJphh9HVzrZ5DPOAR0PDl-qnVys_CRkF0NEwEvAZL5SFumBqjtLBI9XUlWbB6VTljPOExL1zkv8NevZF8DxVsYFaW9HOYH8vNgC07kj_oUVkmAjP-2tVngcBKka0IBmuz2r-RfWNy9VJ-yb19AbtJNw6fjASy7O6VifuH4ZpjP5JSIg'
```

You are also able to deploy a server that the proxy connects to. This is useful
for checking the headers and request body sent to the target server by the
proxy which are present in the server logs. To enable this, set the following
environment variable:

```bash
KUBE_OIDC_PROXY_FAKE_APISERVER=true make dev_cluster_deploy
```

# Delete the cluster

To delete the test kind cluster, use `make dev_cluster_destroy`.
