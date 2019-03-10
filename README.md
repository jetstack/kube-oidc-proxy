# kube-oidc-proxy

kube-oidc-proxy is a reverse proxy server to authenticate users using OIDC for
Kubernetes API servers without OIDC authentication available.

This intermediary server takes kubectl requests, authenticates the request using
the configured OIDC Kubernetes authenticator, attaches impersonation headers
based on the OIDC response from the configured provider with the users provided
token. This impersonated request is then sent to the API server on behalf of the
user and it's response passed back. The server has flag parity with secure
serving and OIDC authentication that are available with the Kubernetes API
server as well as client flags provided by kubectl. In cluster client
authentication is also available when running kube-oidc-proxy in cluster.

Since the proxy server utilises impersonation to forward requests to the API
server once authenticated, impersonation is disabled for user requests to the
API server.

The following is a diagram of the request flow for a user request.
![kube-oidc-proxy request flow](/imgs/kube-oidc-proxy.png)

## Quickstart
This quickstart demo will assume you have a Kubernetes cluster with OIDC
authentication unavailable as well as an OIDC client created with your chosen
provider. We will be using a Service with type LoadBalancer to expose it to the
outside world can be changed depending on what is available and what suites your
set up best.

Firstly deploy `kube-oidc-proxy` and it's related resources into your cluster.
This will create it's Deployment, Service Account and required permissions into
the into the newly created `kube-oidc-proxy` namespace.

```
$ kubectl apply -f ./demo/kube-oidc-porxy.yaml
$ kubectl get all --namespace kube-oidc-proxy
```

This deployment will fail until we create the required secrets. Notice we have
also not provided any client flags as we are using the in cluster config with
it's Service Account.

We now wait until we have an external IP address provisioned.

```
$ kubectl get service --namespace kube-oidc-proxy
```

We need to generate certificates for the kube-oidc-proxy to securely serve.
We will be creating self signed certificates which are tied to either it's IP
address or a domain name that has been configured to point to this address.
These certificates could also be generated through cert-manager, more
information about this project found
[here](https://github.com/jetstack/cert-manager).

```
$ ./demo/gencreds.sh kube-oidc-proxy ${kube-oidc-proxy_IP}
```

or

```
$ ./demo/gencreds.sh k8s.my-domain.com
```

This should generate a certificate authority along with a signed key pair for
use by kube-oidc-proxy in `./demo/generated`. Enter the TLS key and certificate
into the secure serving Kubernetes Secret manifest.

```
$ SERVING_TLS_CERT=$(cat ./demo/generated/kube-oidc-proxy-cert.pem | base64 -w0); sed -i -e "s/SERVING_TLS_CERT/${SERVING_TLS_CERT}/g" ./demo/secrets.yaml
$ SERVING_TLS_KEY=$(cat ./demo/generated/kube-oidc-proxy-key.pem | base64 -w0); sed -i -e "s/SERVING_TLS_KEY/${SERVING_TLS_KEY}/g" ./demo/secrets.yaml
```

Next, populate the OIDC authenticator Secret using the secrets given to you
by your OIDC provider in `./demo/secrets.yaml`. The OIDC provider CA will be
different depending on which provider you are using. The easiest way to obtain
the correct certificate bundle is often by opening the providers URL into a
browser and fetching them there (typically output by clicking the lock icon on
your address bar). Google's OIDC provider for example requires CAs from both
`https://accounts.google.com/.well-known/openid-configuration` and
`https://www.googleapis.com/oauth2/v3/certs`.


Apply the two secret manifests.

```
kubectl apply -f ./demo/secrets.yaml
```

You may need to also recreate the `kube-oidc-proxy` pod to use these new secrets
now they are available.

```
kubectl delete pod --namespace kube-oidc-proxy kube-oidc-proxy-*
```

Finally, create a Kubeconfig to now point to kube-oidc-proxy as well as setting
up your OIDC authenticated Kubernetes user.

```
apiVersion: v1
clusters:
- cluster:
    certificate-authority: ./demo/generated/ca.pem
    server: https://[url|ip:443]
  name: *
contexts:
- context:
    cluster: *
    user: *
  name: *
kind: Config
preferences: {}
users:
- name: *
  user:
    auth-provider:
      config:
        client-id: *
        client-secret: *
        id-token: *
        idp-issuer-url: *
        refresh-token: *
      name: oidc
```

## Development
*NOTE*: building kube-oidc-proxy requires Go version 1.12 or higher.
