# kube-oidc-proxy

>  :warning:
>
>  kube-oidc-proxy is an experimental tool that we would like to get feedback
>  on from the community. Jetstack makes no guarantees on the soundness of the
>  security in this project, nor any suggestion that it's 'production ready'.
>  This server sits in the critical path of authentication to the Kubernetes
>  API.
>
>  :warning:

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

![kube-oidc-proxy demo](https://storage.googleapis.com/kube-oidc-proxy/demo-9de755f8e4b4e5dd67d17addf09759860f903098.svg)

The following is a diagram of the request flow for a user request.
![kube-oidc-proxy request
flow](https://storage.googleapis.com/kube-oidc-proxy/diagram-d9623e38a6cd3b585b45f47d80ca1e1c43c7e695.png)

## Tutorial

Directions on how to deploy OIDC authentication with multi-cluster can be found
[here.](./demo/README.md) or there is a [helm chart](./deploy/charts/kube-oidc-proxy/README.md).

### Quickstart

Deployment yamls can be found in `./deploy/yaml` and will require configuration to
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
$ kubectl apply -f ./deploy/yaml/kube-oidc-proxy.yaml
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
by your OIDC provider in `./deploy/yaml/secrets.yaml`. The OIDC provider CA will be
different depending on which provider you are using. The easiest way to obtain
the correct certificate bundle is often by opening the providers URL into a
browser and fetching them there (typically output by clicking the lock icon on
your address bar). Google's OIDC provider for example requires CAs from both
`https://accounts.google.com/.well-known/openid-configuration` and
`https://www.googleapis.com/oauth2/v3/certs`.


Apply the secret manifests.

```
kubectl apply -f ./deploy/yaml/secrets.yaml
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

## Configuration
 - [Token Passthrough](./docs/tasks/token-passthrough.md)
 - [No Impersonation](./docs/tasks/no-impersonation.md)
 - [Extra Impersonations Headers](./docs/tasks/extra-impersonation-headers.md)
 - [Auditing](./docs/tasks/auditing.md)

## Logging

In addition to auditing, kube-oidc-proxy logs all requests to standard out so the requests can be captured by a common Security Information and Event Management (SIEM) system.  SIEMs will typically import logs directly from containers via tools like fluentd.  This logging is also useful in debugging.  An example successful event:

```
[2021-11-25T01:05:17+0000] AuSuccess src:[10.42.0.5 / 10.42.1.3, 10.42.0.5] URI:/api/v1/namespaces/openunison/pods?limit=500 inbound:[mlbadmin1 / system:masters|system:authenticated /]
```

The first block, between `[]` is an ISO-8601 timestamp.  The next text, `AuSuccess`, indicates that authentication was successful.  the `src` block containers the remote address of the request, followed by the value of the `X-Forwarded-For` HTTP header if provided.  The `URI` is the URL path of the request.  The `inbound` section provides the user name, groups, and extra-info provided to the proxy from the JWT.

When there's an error or failure:

```
[2021-11-25T01:05:24+0000] AuFail src:[10.42.0.5 / 10.42.1.3] URI:/api/v1/nodes
```

This is similar to success, but without the token information.

## End-User Impersonation

kube-oidc-proxy supports the impersonation headers for inbound requests.  This allowes the proxy to support `kubectl --as`.  When impersonation headers are included in a request, the proxy checks that the authenticated user is able to assume the identity of the impersonation headers by submitting `SubjectAccessReview` requests to the API server.  Once authorized, the proxy will send those identity headers instead of headers generated for the authenticated user.  In addition, three `Extra` impersonation headers are sent to the API server to identify the authenticated user who's making the request:

| Header | Description |
| ------ | ----------- |
| `originaluser.jetstack.io-user` | The original username |
| `originaluser.jetstack.io-groups` | The original groups |
| `originaluser.jetstack.io-extra` | A JSON encoded map of arrays representing all of the `extra` headers included in the original identity |

In addition to sending this `extra` information, the proxy adds an additional section to the logfile that will identify outbound identity data.  When impersonation headers are present, the `AuSuccess` log will look like:

```
[2021-11-25T01:05:17+0000] AuSuccess src:[10.42.0.5 / 10.42.1.3, 10.42.0.5] URI:/api/v1/namespaces/openunison/pods?limit=500 inbound:[mlbadmin1 / system:masters|system:authenticated /] outbound:[mlbadmin2 / group2|system:authenticated /]
```

When using `Impersonate-Extra-` headers, the proxy's `ServiceAccount` must be explicitly authorized via RBAC to impersonate whatever the extra key is named.  This is because extras are treated as subresources which must be explicitly authorized.  


## Development
*NOTE*: building kube-oidc-proxy requires Go version 1.12 or higher.

To help with development, there is a suite of tools you can use to deploy a
functioning proxy from source locally. You can read more
[here](./docs/tasks/development-testing.md).
