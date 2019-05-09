local upstream_cert_manager = import '../vendor/kube-prod-runtime/components/cert-manager.jsonnet';
local kube = import '../vendor/kube-prod-runtime/lib/kube.libsonnet';

local CERT_MANAGER_IMAGE = '';

local add_acme_spec(issuer, obj) =
  if std.objectHas(issuer.spec, 'acme') then
    obj {
      spec+: {
        acme: {
          config: [{
            dns01: {
              provider: issuer.spec.acme.dns01.providers[0].name,
            },
            domains: obj.spec.dnsNames,
          }],
        },
      },
    }
  else
    obj;

upstream_cert_manager {
  ca_secret_name:: 'ca-key-pair',

  // create simple to use certificate resource
  Certificate(namespace, name, issuer, domains):: add_acme_spec(issuer, kube._Object($.certCRD.spec.group + '/' + $.certCRD.spec.version, $.certCRD.spec.names.kind, name) + {
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
    },
  }),

  // TODO: use upstream images for cert-manager
}
