# kube-oidc-proxy

`kube-oidc-proxy` is a reverse proxy server to authenticate users using OIDC to
Kubernetes API servers where OIDC authentication is not available (i.e. managed 
Kubernetes providers such as GKE, EKS, etc).

This intermediary server takes `kubectl` requests, authenticates the request using
the configured OIDC Kubernetes authenticator, then attaches impersonation
headers based on the OIDC response from the configured provider. This
impersonated request is then sent to the API server on behalf of the user and
it's response passed back. The server has flag parity with secure serving and
OIDC authentication that are available with the Kubernetes API server as well as
client flags provided by kubectl. In-cluster client authentication is also
available when running `kube-oidc-proxy` as a pod.

Since the proxy server utilises impersonation to forward requests to the API
server once authenticated, impersonation is disabled for user requests to the
API server.

![kube-oidc-proxy demo](https://storage.googleapis.com/kube-oidc-proxy/demo.svg)

The following is a diagram of the request flow for a user request.
![kube-oidc-proxy request flow](/img/kube-oidc-proxy.png)

## Tutorial

Directions on how to deploy OIDC authentication with multi-cluster can be found
[here.](./demo/README.md)

### Quickstart

Deployment yamls can be found in `./demo/yaml` and will require configuration to
an exiting OIDC issuer.

This quickstart demo will assume you have a Kubernetes cluster without OIDC
authentication, as well as an OIDC client created with your chosen
provider. We will be using a Service with type `LoadBalancer` to expose it to
the outside world. This can be changed depending on what is available and what
suites your set up best.

Firstly deploy `kube-oidc-proxy` and it's related resources into your cluster.
This will create it's Deployment, Service Account and required permissions into
the newly created `kube-oidc-proxy` Namespace.

```
$ kubectl apply -f ./demo/yaml/kube-oidc-proxy.yaml
$ kubectl get all --namespace kube-oidc-proxy
```

This deployment will fail until we create the required secrets. Notice we have
also not provided any client flags as we are using the in-cluster config with
it's Service Account.

We now wait until we have an external IP address provisioned.

```
$ kubectl get service --namespace kube-oidc-proxy
```

We need to generate certificates for `kube-oidc-proxy` to securely serve.  These
certificates can be generated through `cert-manager`, more information about
this project found [here](https://github.com/jetstack/cert-manager).

Next, populate the OIDC authenticator Secret using the secrets given to you
by your OIDC provider in `./demo/yaml/secrets.yaml`. The OIDC provider CA will be
different depending on which provider you are using. The easiest way to obtain
the correct certificate bundle is often by opening the providers URL into a
browser and fetching them there (typically output by clicking the lock icon on
your address bar). Google's OIDC provider for example requires CAs from both
`https://accounts.google.com/.well-known/openid-configuration` and
`https://www.googleapis.com/oauth2/v3/certs`.


Apply the secret manifests.

```
kubectl apply -f ./demo/yaml/secrets.yaml
```

You can restart the `kube-oidc-proxy` pod to use these new secrets
now they are available.

```
kubectl delete pod --namespace kube-oidc-proxy kube-oidc-proxy-*
```

Finally, create a Kubeconfig to point to `kube-oidc-proxy` and set up your OIDC
authenticated Kubernetes user.

```
apiVersion: v1
clusters:
- cluster:
    certificate-authority: *
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
