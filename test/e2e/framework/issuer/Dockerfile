# Copyright Jetstack Ltd. See LICENSE for details.
FROM alpine:3.10

LABEL description="A basic OIDC issuer that prsents a well-known and certs endpoint."

RUN apk --no-cache --update add ca-certificates

COPY ./bin/oidc-issuer /oidc-issuer

ENTRYPOINT ["/oidc-issuer"]
