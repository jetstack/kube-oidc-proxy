# Copyright Jetstack Ltd. See LICENSE for details.
FROM alpine:3.9
LABEL description="OIDC reverse proxy authenticator based on Kubernetes"

RUN apk --no-cache --update add ca-certificates

COPY ./bin/kube-oidc-proxy /usr/bin/kube-oidc-proxy

CMD ["/usr/bin/kube-oidc-proxy"]
