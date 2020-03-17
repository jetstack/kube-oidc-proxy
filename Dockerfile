# Copyright Jetstack Ltd. See LICENSE for details.
FROM alpine:3.10
LABEL description="OIDC reverse proxy authenticator based on Kubernetes"

RUN apk --no-cache --update add ca-certificates

COPY ./bin/kube-oidc-proxy-linux /usr/bin/kube-oidc-proxy

CMD ["/usr/bin/kube-oidc-proxy"]
