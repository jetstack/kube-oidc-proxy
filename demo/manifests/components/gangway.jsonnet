local kube = import '../vendor/kube-prod-runtime/lib/kube.libsonnet';
local utils = import '../vendor/kube-prod-runtime/lib/utils.libsonnet';

local GANGWAY_IMAGE = 'gcr.io/heptio-images/gangway:v3.0.0';
local GANGWAY_CONFIG_PATH = '/etc/gangway';

{
  p:: '',

  sessionSecurityKey:: error 'sessionSecurityKey is undefined',

  base_domain:: 'cluster.local',

  app:: 'gangway',
  domain:: $.app + '.' + $.base_domain,

  namespace:: 'gangway',

  secret_key:: '',
  client_secret:: '',

  gangway_url:: 'https://' + $.domain,
  kubernetes_url:: 'https://kubernetes-api.' + $.base_domain,
  authorize_url:: 'https://' + $.domain + '/dex/auth',
  token_url:: 'https://' + $.domain + '/dex/token',
  cluster_name:: 'my-cluster',

  port:: 8080,

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
    usernameClaim: 'sub',
    redirectURL: $.gangway_url + '/callback',
    clusterName: $.cluster_name,
    authorizeURL: $.authorize_url,
    tokenURL: $.token_url,
    clientID: 'gangway',
    serveTLS: true,
    certFile: GANGWAY_CONFIG_PATH + '/tls/tls.crt',
    keyFile: GANGWAY_CONFIG_PATH + '/tls/tls.key',
  },


  configMap: kube.ConfigMap($.p + $.app) + $.metadata {
    data+: {
      'gangway.yaml': std.manifestJsonEx($.config, '  '),
    },
  },

  secret: kube.Secret($.p + $.app) + $.metadata {
    data+: {
      'session-security-key': $.secret_key,
      'client-secret': $.client_secret,
    },
  },

  deployment: kube.Deployment($.p + $.app) + $.metadata {
    local this = self,
    spec+: {
      replicas: 3,
      template+: {
        metadata+: {
          annotations+: {
            'config/hash': std.md5(std.escapeStringJson($.configMap)),
            'secret/hash': std.md5(std.escapeStringJson($.secret)),
          },
        },
        spec+: {
          affinity: kube.PodZoneAntiAffinityAnnotation(this.spec.template),
          default_container: $.app,
          volumes_+: {
            secret: kube.SecretVolume($.secret),
            config: kube.ConfigMapVolume($.configMap),
            tls: {
              secret: {
                secretName: $.p + $.app + '-tls',
              },
            },
          },
          containers_+: {
            gangway: kube.Container($.app) {
              image: GANGWAY_IMAGE,
              command: [$.app],
              args: [
                '-config',
                GANGWAY_CONFIG_PATH + '/gangway.yaml',
              ],
              ports_+: {
                http: { containerPort: $.port },
              },
              env_+: {
                GANGWAY_PORT: $.port,
                GANGWAY_SESSION_SECURITY_KEY: kube.SecretKeyRef($.secret, "session-security-key"),
                GANGWAY_CLIENT_SECRET: kube.SecretKeyRef($.secret, "client-secret"),
              },
              readinessProbe: {
                httpGet: { path: '/', port: $.port, scheme: "HTTPS" },
                periodSeconds: 10,
                failureThreshold: 3,
                timeoutSeconds: 1,
              },
              livenessProbe: {
                httpGet: { path: '/', port: $.port, scheme: "HTTPS" },
                initialDelaySeconds: 20,
                timeoutSeconds: 1,
                periodSeconds: 60,
                failureThreshold: 3,
              },
              volumeMounts_+: {
                config: { mountPath: GANGWAY_CONFIG_PATH, readOnly: true },
                tls: { mountPath: GANGWAY_CONFIG_PATH + '/tls' , readOnly: true },
              },
            },
          },
        },
      },
    },
  },

  svc: kube.Service($.p + $.app) + $.metadata {
    target_pod: $.deployment.spec.template,
  },
}
