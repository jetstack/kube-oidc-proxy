# Extra Impersonation Headers

kube-oidc-proxy has support for adding 'extra' headers to the impersonation user
info. This can be useful for passing extra information onto the target server
about the proxy or client. kube-oidc-proxy currently supports two configuration
options.

# Client IP

The following flag can be passed which will append the remote client IP as an
extra header:

`--extra-user-header-client-ip`

Proxied requests will then contain the header
`Impersonate-Extra-Remote-Client-Ip: <REMOTE_ADDR>` where  `<REMOTE_ADDR>` is
the address of the client that made the request.

# Extra User Headers

The following flag accepts a number of key value pairs that will be added as
extra impersonation headers with proxied requests. This flag accepts a number of
key value pairs, separated by commas, where a single key may have multiple
values:

`--extra-user-headers=key1=foo,key2=bar,key1=bar`

Proxied requests will then contain the headers

`Impersonate-Extra-Key1: foo,bar`
`Impersonate-Extra-Key2: foo`
