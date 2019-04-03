local upstream_cert_manager = import '../vendor/kube-prod-runtime/components/cert-manager.jsonnet';
local kube = import '../vendor/kube-prod-runtime/lib/kube.libsonnet';

local CERT_MANAGER_IMAGE = '';

upstream_cert_manager {
  // create simple to use certificate resource
  Certificate(namespace, name, issuer, domains):: kube._Object($.certCRD.spec.group + '/' + $.certCRD.spec.version, $.certCRD.spec.names.kind, name) + {
    metadata+: {
      namespace: namespace,
      name: name,
    },
    spec+: {
      secretName: name + '-tls',
      dnsNames: domains,
      issuerRef: {
        name: issuer.metadata.name,
        kind: issuer.kind,
      },
      acme: {
        config: [
          {
            dns01: {
              provider: issuer.spec.acme.dns01.providers[0].name,
            },
            domains: domains,
          },
        ],
      },
    },
  },

  // TODO: use upstream images for cert-manager
}
