local kube = import '../vendor/kube-prod-runtime/lib/kube.libsonnet';
local cert_manager_manifests = import './cert-manager/cert-manager.json';
local apiGroup = 'cert-manager.io/v1alpha2';

{
  ca_secret_name:: 'ca-key-pair',

  p:: '',
  metadata:: {
    metadata+: {
      namespace: 'kubeprod',
    },
  },
  letsencrypt_contact_email:: error 'Letsencrypt contact e-mail is undefined',

  // create simple to use certificate resource
  Certificate(namespace, name, issuer, solver, domains):: kube._Object(apiGroup, 'Certificate', name) + {
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
  },

  // Letsencrypt environments
  letsencrypt_environments:: {
    prod: $.letsencryptProd.metadata.name,
    staging: $.letsencryptStaging.metadata.name,
  },
  // Letsencrypt environment (defaults to the production one)
  letsencrypt_environment:: 'prod',

  Issuer(name):: kube._Object(apiGroup, 'Issuer', name) {
  },

  ClusterIssuer(name):: kube._Object(apiGroup, 'ClusterIssuer', name) {
  },

  deploy: cert_manager_manifests,

  letsencryptStaging: $.ClusterIssuer($.p + 'letsencrypt-staging') {
    local this = self,
    spec+: {
      acme+: {
        server: 'https://acme-staging-v02.api.letsencrypt.org/directory',
        email: $.letsencrypt_contact_email,
        privateKeySecretRef: { name: this.metadata.name },
        http01: {},
      },
    },
  },

  letsencryptProd: $.letsencryptStaging {
    metadata+: { name: $.p + 'letsencrypt-prod' },
    spec+: {
      acme+: {
        server: 'https://acme-v02.api.letsencrypt.org/directory',
      },
    },
  },

  solvers+:: [],
}
