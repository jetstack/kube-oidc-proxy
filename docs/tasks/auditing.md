# Auditing

kube-oidc-proxy allows for the ability to audit requests to the proxy. The proxy
exposes all the same options for auditing that the Kubernetes API server
provides, however does _not_ support dynamic configuration
(`--audit-dynamic-configuration`).

You can read more on how to configure and manage auditing in the [Kubernetes
documentation](https://kubernetes.io/docs/tasks/debug-application-cluster/audit).
