# Copyright Jetstack Ltd. See LICENSE for details.
FROM alpine:3.9

RUN apk --no-cache --update add ca-certificates

COPY kube-oidc-proxy /usr/bin/

CMD ["/usr/bin/kube-oidc-proxy"]
