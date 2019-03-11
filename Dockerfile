# Copyright Jetstack Ltd. See LICENSE for details.
from alpine:latest

COPY kube-oidc-proxy /

ENTRYPOINT ["/kube-oidc-proxy"]
