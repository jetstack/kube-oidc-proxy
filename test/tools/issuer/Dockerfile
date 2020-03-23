# Copyright Jetstack Ltd. See LICENSE for details.
FROM alpine:3.10

LABEL description="A basic OIDC issuer that prsents a well-known and certs endpoint."

RUN apk --no-cache add ca-certificates

COPY ./bin/oidc-issuer-linux /usr/bin/oidc-issuer

CMD ["/usr/bin/oidc-issuer"]
