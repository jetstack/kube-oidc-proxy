# kube-oidc-proxy helm chart

This is a `helm` chart that installs [`kube-oidc-proxy`](https://github.com/jetstack/kube-oidc-proxy/).
This helm chart cannot be installed out of the box without providing own
configuration.

This helm chart is based on example configuration provided in `kube-oidc-proxy`
[repository](https://github.com/jetstack/kube-oidc-proxy/blob/master/deploy/yaml/kube-oidc-proxy.yaml).

Minimal required configuration is `oidc` section of `value.yaml` file.

```yaml
oidc:
  clientId: my-client
  issuerUrl: https://accounts.google.com
  usernameClaim: email
```

When a custom root CA certificate is required it should be added as PEM encoded
text value:

```yaml
oidc:
  caPEM: |
    -----BEGIN CERTIFICATE-----
    MIIDdTCCAl2gAwIBAgILBAAAAAABFUtaw5QwDQYJKoZIhvcNAQEFBQAwVzELMAkG
    A1UEBhMCQkUxGTAXBgNVBAoTEEdsb2JhbFNpZ24gbnYtc2ExEDAOBgNVBAsTB1Jv
    b3QgQ0ExGzAZBgNVBAMTEkdsb2JhbFNpZ24gUm9vdCBDQTAeFw05ODA5MDExMjAw
    MDBaFw0yODAxMjgxMjAwMDBaMFcxCzAJBgNVBAYTAkJFMRkwFwYDVQQKExBHbG9i
    YWxTaWduIG52LXNhMRAwDgYDVQQLEwdSb290IENBMRswGQYDVQQDExJHbG9iYWxT
    aWduIFJvb3QgQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDaDuaZ
    jc6j40+Kfvvxi4Mla+pIH/EqsLmVEQS98GPR4mdmzxzdzxtIK+6NiY6arymAZavp
    xy0Sy6scTHAHoT0KMM0VjU/43dSMUBUc71DuxC73/OlS8pF94G3VNTCOXkNz8kHp
    1Wrjsok6Vjk4bwY8iGlbKk3Fp1S4bInMm/k8yuX9ifUSPJJ4ltbcdG6TRGHRjcdG
    snUOhugZitVtbNV4FpWi6cgKOOvyJBNPc1STE4U6G7weNLWLBYy5d4ux2x8gkasJ
    U26Qzns3dLlwR5EiUWMWea6xrkEmCMgZK9FGqkjWZCrXgzT/LCrBbBlDSgeF59N8
    9iFo7+ryUp9/k5DPAgMBAAGjQjBAMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8E
    BTADAQH/MB0GA1UdDgQWBBRge2YaRQ2XyolQL30EzTSo//z9SzANBgkqhkiG9w0B
    AQUFAAOCAQEA1nPnfE920I2/7LqivjTFKDK1fPxsnCwrvQmeU79rXqoRSLblCKOz
    yj1hTdNGCbM+w6DjY1Ub8rrvrTnhQ7k4o+YviiY776BQVvnGCv04zcQLcFGUl5gE
    38NflNUVyRRBnMRddWQVDf9VMOyGj/8N7yy5Y0b2qvzfvGn9LhJIZJrglfCm7ymP
    AbEVtQwdpf5pLGkkeB6zpxxxYu7KyJesF12KwvhHhm4qxFYxldBniYUr+WymXUad
    DKqC5JlR3XC321Y9YeRq4VzW9v493kHMB65jUr9TU/Qr6cf9tveCX4XSQRjbgbME
    HMUfpIBvFSDJ3gyICh3WZlXi/EjJKSZp4A==
    -----END CERTIFICATE-----
```

This minimal configuration gives a cluster internal IP address that can be used
with `kubectl` to authenticate requests to Kubernetes API server.

The service can be exposed via ingress controller and give access to external
clients. Example of exposing via ingress controller.

```yaml
ingress:
  enabled: true
  annotations:
    kubernetes.io/ingress.class: traefik
    traefik.ingress.kubernetes.io/rule-type: PathPrefixStrip
  hosts:
    - host: ""
      paths:
        - /oidc-proxy
```

By default the helm chart will create self-signed TLS certificate for `kube-oidc-proxy`
service. It is possible to provide secret name that contains TLS artifacts for
service. The secret must be of `kubernetes.io/tls` type.

```yaml
tls:
  secretName: my-tls-secret-with-key-and-cert
```
