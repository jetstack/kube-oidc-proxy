# WARNING: Copied Package

This package has been taken from
`k8s.io/kubernetes/pkg/serviceaccount:92b2e906d7aa618588167817feaed137a44e6d92`
(release-1.15) with modifications to remove dependencies in files with
`k8s.io/kubernetes` as so:

- `k8s.io/kubernetes/pkg/apis/core/` => `k8s.io/api/core/v1`
- `k8s.io/kubernetes/pkg/controller/serviceaccount` =>
  `github.com/jetstack/kube-oidc-proxy/pkg/proxy/serviceaccount/getter`


Ideally this package will be moved into a staging repository and can be properly
vendored.
