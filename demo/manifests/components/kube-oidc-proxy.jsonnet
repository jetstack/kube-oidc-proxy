local kube = import '../vendor/kube-prod-runtime/lib/kube.libsonnet';
local utils = import '../vendor/kube-prod-runtime/lib/utils.libsonnet';

local kube_oidc_proxy_clusterrole = import 'kube-oidc-proxy-clusterrole.json';

local KUBE_OIDC_PROXY_IMAGE = 'docker.io/simonswine/kube-oidc-proxy';
local CONFIG_PATH = '/etc/kube-oidc-proxy';
local READINESS_PORT = 8080;

{
  p:: '',

  base_domain:: 'example.net',

  app:: 'kube-oidc-proxy',
  domain:: $.app + '.' + $.base_domain,

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
      tlsCertFile: CONFIG_PATH + '/tls/tls.crt',
      tlsKeyFile: CONFIG_PATH + '/tls/tls.key',
      port: 443,
    },

    oidc+: {
      clientID: 'kube-oidc-proxy',
      usernameClaim: 'email',
      issuerURL: 'https://myprovider',
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
            } +
            if std.objectHas($.config.oidc, 'ca') then
              { 'oidc.ca-pem': $.config.oidc.ca }
            else {},
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
                probe: { containerPort: READINESS_PORT },
              },

              readinessProbe: {
                tcpSocket: { port: READINESS_PORT },
                initialDelaySeconds: 15,
                periodSeconds: 10,
              },
              command: [
                'kube-oidc-proxy',
                '--secure-port=' + $.config.secureServing.port,
                '--tls-cert-file=' + $.config.secureServing.tlsCertFile,
                '--tls-private-key-file=' + $.config.secureServing.tlsKeyFile,
                '--oidc-client-id=$(OIDC_CLIENT_ID)',
                '--oidc-issuer-url=$(OIDC_ISSUER_URL)',
              ] + if std.objectHas($.config.oidc, 'caFile') then
                ['--oidc-ca-file=' + $.config.oidc.caFile]
              else
                [] + if std.objectHas($.config.oidc, 'usernameClaim') then
                  ['--oidc-username-claim=' + $.config.oidc.usernameClaim]
                else
                  []
              ,

              env_+: {
                OIDC_CLIENT_ID: kube.SecretKeyRef($.oidc_secret, 'oidc.client-id'),
                OIDC_ISSUER_URL: kube.SecretKeyRef($.oidc_secret, 'oidc.issuer-url'),
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
    port: $.config.secureServing.port,

    spec+: {
      ports: [{
        port: $.config.secureServing.port,
        targetPort: $.config.secureServing.port,
      }],

      sessionAffinity: 'None',
    },
  },
}
