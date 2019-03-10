from alpine:latest

COPY kube-oidc-proxy /

ENTRYPOINT ["/kube-oidc-proxy"]
