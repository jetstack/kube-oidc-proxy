# Copyright Jetstack Ltd. See LICENSE for details.
FROM alpine:3.10

LABEL description="A audit webhook sink to read audit events and write to file."

RUN apk --no-cache add ca-certificates

COPY ./bin/audit-webhook /usr/bin/audit-webhook

CMD ["/usr/bin/audit-webhook"]
