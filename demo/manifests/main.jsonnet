local kube = import './vendor/kube-prod-runtime/lib/kube.libsonnet';

local cert_manager = import './vendor/kube-prod-runtime/components/cert-manager.jsonnet';
local externaldns = import './vendor/kube-prod-runtime/components/externaldns.jsonnet';

local contour = import './components/contour.jsonnet';
local dex = import './components/dex.jsonnet';
local gangway = import './components/gangway.jsonnet';

local config = import './config.json';

local base_domain = 'lab.christian-gcp.jetstack.net';
local namespace = 'auth';

{
  config:: config,

  cert_manager: cert_manager {
    metadata:: {
      metadata+: {
        namespace: 'kube-system',
      },
    },
    letsencrypt_contact_email:: 'simon+letsencrypt@swine.de',
    letsencrypt_environment:: 'prod',
  },

  cert_manager_google_secret: kube.Secret($.cert_manager.p + 'clouddns-google-credentials') + $.cert_manager.metadata {
    data_+: {
      'credentials.json': $.config.externaldns.service_account_credentials,
    },
  },
  cert_manager_google_issuer: cert_manager.Issuer('clouddns') {
  },

  externaldns: externaldns {
    metadata:: {
      metadata+: {
        namespace: 'kube-system',
      },
    },

    gcreds: kube.Secret($.externaldns.p + 'externaldns-google-credentials') + $.externaldns.metadata {
      data_+: {
        'credentials.json': $.config.externaldns.service_account_credentials,
      },
    },

    deploy+: {
      ownerId: base_domain,
      spec+: {
        template+: {
          spec+: {
            volumes_+: {
              gcreds: kube.SecretVolume($.externaldns.gcreds),
            },
            containers_+: {
              edns+: {
                args_+: {
                  provider: 'google',
                  'google-project': $.config.externaldns.project,
                },
                env_+: {
                  GOOGLE_APPLICATION_CREDENTIALS: '/google/credentials.json',
                },
                volumeMounts_+: {
                  gcreds: { mountPath: '/google', readOnly: true },
                },
              },
            },
          },
        },
      },
    },
  },

  namespace: kube.Namespace(namespace),

  contour: contour {
    metadata:: {
      metadata+: {
        namespace: namespace,
      },
    },
  },

  dex: dex {
    namespace:: namespace,
    base_domain:: base_domain,
  },
  dexPasswordChristian: dex.Password('christian', 'simon@swine.de', '$2y$10$i2.tSLkchjnpvnI73iSW/OPAVriV9BWbdfM6qemBM1buNRu81.ZG.'),  // plaintext: secure
  dexIngress: {},

  gangway: gangway {
    metadata:: {
      metadata+: {
        namespace: namespace,
      },
    },
  },

}
