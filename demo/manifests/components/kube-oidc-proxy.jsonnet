local kube = import '../vendor/kube-prod-runtime/lib/kube.libsonnet';
local utils = import '../vendor/kube-prod-runtime/lib/utils.libsonnet';

local kube_oidc_proxy_clusterrole = import 'kube-oidc-proxy-clusterrole.json';

local KUBE_OIDC_PROXY_IMAGE = 'docker.io/simonswine/kube-oidc-proxy';
local CONFIG_PATH = '/etc/kube-oidc-proxy';

{
  p:: '',

  base_domain:: 'example.net',

  app:: 'kube-oidc-proxy',
  domain:: $.app + '.' + $.base_domain,

  oidc_issuer_url:: 'https://dex.' + $.base_domain,
  oidc_client_id:: '',
  oidc_username_claim:: 'Username',
  oidc_ca:: '',
  oidc_ca_file:: CONFIG_PATH + '/oidc/ca.pem',

  tls_cert_file:: CONFIG_PATH + '/tls/tls.crt',
  tls_key_file:: CONFIG_PATH + '/tls/tls.key',
  secure_serving_port:: 443,
  readiness_port:: 8080,

  namespace:: 'kube-oidc-proxy',

  labels:: {
    metadata+: {
      labels+: {
        app: $.app,
      },
    },
  },

  metadata:: $.labels {
    metadata+: {
      namespace: $.namespace,
    },
  },

  config:: {
    secureServing+: {
      tlsCertFile: $.tls_cert_file,
      tlsKeyFile: $.tls_key_file,
      port: $.secure_serving_port,
    },

    oidc+: {
      clientID: $.oidc_client_id,
      issuerURL: $.oidc_issuer_url,
      usernameClaim: $.oidc_username_claim,
      caFile: $.oidc_ca_file,
      ca: $.oidc_ca,
    },
  },

  clusterRole: kube_oidc_proxy_clusterrole + $.labels,

  serviceAccount: kube.ServiceAccount($.p + $.app) + $.metadata {
  },

  clusterRoleBinding: kube.ClusterRoleBinding($.p + $.app) + $.metadata {
    roleRef_: $.clusterRole,
    subjects_+: [$.serviceAccount],
  },

  oidc_secret: kube.Secret($.p + 'kube-oidc-proxy-config') + $.metadata {
    data_+: {
      'oidc.client-id': $.config.oidc.clientID,
      'oidc.issuer-url': $.config.oidc.issuerURL,
      'oidc.username-claim': $.config.oidc.usernameClaim,
      'oidc.ca-pem': $.config.oidc.ca,
    },
  },

  deployment: kube.Deployment($.p + $.app) + $.metadata {
    local this = self,

    spec+: {
      replicas: 3,
      template+: {
        spec+: {
          serviceAccountName: $.serviceAccount.metadata.name,
          containers_+: {
            kubeOIDCProxy: kube.Container($.app) {
              image: KUBE_OIDC_PROXY_IMAGE,

              ports_+: {
                serving: { containerPort: $.config.secureServing.port },
                probe: { containerPort: $.readiness_port },
              },

              readinessProbe: {
                tcpSocket: { port: $.readiness_port },
                initialDelaySeconds: 15,
                periodSeconds: 10,
              },

              args: [
                '--secure-port=' + $.config.secureServing.port,
                '--tls-cert-file=' + $.config.secureServing.tlsCertFile,
                '--tls-private-key-file=' + $.config.secureServing.tlsKeyFile,
                '--oidc-client-id=$(OIDC_CLIENT_ID)',
                '--oidc-issuer-url=$(OIDC_ISSUER_URL)',
                '--oidc-username-claim=$(OIDC_USERNAME_CLAIM)',
                '--oidc-ca-file=' + $.config.oidc.caFile,
              ],

              env_+: {
                OIDC_CLIENT_ID: kube.SecretKeyRef($.oidc_secret, "oidc.client-id"),
                OIDC_ISSUER_URL: kube.SecretKeyRef($.oidc_secret, "oidc.issuer-url"),
                OIDC_USERNAME_CLAIM: kube.SecretKeyRef($.oidc_secret, "oidc.username-claim"),
              },

              volumeMounts_+: {
                oidc: { mountPath: CONFIG_PATH + '/oidc', readOnly: true },
                serving: { mountPath: CONFIG_PATH + '/tls', readOnly: true },
              },
            },
          },

          volumes_+: {
            oidc: kube.SecretVolume($.oidc_secret),
            serving: {
              secret: {
                secretName: $.p + $.app + '-tls',
              },
            },
          },
        },
      },
    },
  },

  svc: kube.Service($.p + $.app) + $.metadata {
    target_pod: $.deployment.spec.template,
    port: $.secure_serving_port,

    spec+: {
      ports: [{
        port: $.secure_serving_port,
        targetPort: $.secure_serving_port,
      }],

      sessionAffinity: 'None',
    },
  },
}
