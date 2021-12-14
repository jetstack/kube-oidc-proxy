# Copyright Jetstack Ltd. See LICENSE for details.
FROM alpine:latest
LABEL description="OIDC reverse proxy authenticator based on Kubernetes"

RUN apk --no-cache add ca-certificates \
    && apk --no-cache add --upgrade openssl

COPY ./bin/kube-oidc-proxy /usr/bin/kube-oidc-proxy

CMD ["/usr/bin/kube-oidc-proxy"]
