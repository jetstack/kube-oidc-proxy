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

  client_secret:: '',

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

  secret: kube.Secret($.p + $.app) + $.metadata {
    data_+: {
      'client-id': $.config.oidc.clientID,
      'issuer-url': $.config.oidc.issuerURL,
      'username-claim': $.config.oidc.usernameClaim,
      } +
      if std.objectHas($.config.oidc, 'ca') then
        { 'ca-pem': $.config.oidc.ca }
      else {},
  },

  deployment: kube.Deployment($.p + $.app) + $.metadata {
    local this = self,

    metadata+: {
      annotations+: {
        'secret/hash': std.md5(std.escapeStringJson($.client_secret)),
      },
    },

    spec+: {
      replicas: 3,
      template+: {
        metadata+: {
          annotations+: {
            'secret/hash': std.md5(std.escapeStringJson($.secret)),
          },
        },
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
                OIDC_CLIENT_ID: kube.SecretKeyRef($.secret, "client-id"),
                OIDC_ISSUER_URL: kube.SecretKeyRef($.secret, "issuer-url"),
                OIDC_USERNAME_CLAIM: kube.SecretKeyRef($.secret, "username-claim"),
              },

              volumeMounts_+: {
                oidc: { mountPath: CONFIG_PATH + '/oidc', readOnly: true },
                tls: { mountPath: CONFIG_PATH + '/tls', readOnly: true },
              },
            },
          },

          volumes_+: {
            oidc: kube.SecretVolume($.secret),
            tls: {
              secret: { secretName: $.p + $.app + '-tls', },
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
