# Copyright Jetstack Ltd. See LICENSE for details.
FROM alpine:3.10

LABEL description="A fake API server that will respond to requests with the same body and headers."

RUN apk --no-cache add ca-certificates

COPY ./bin/fake-apiserver-linux /usr/bin/fake-apiserver

CMD ["/usr/bin/fake-apiserver"]
