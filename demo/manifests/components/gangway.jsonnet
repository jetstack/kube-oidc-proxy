local kube = import '../vendor/kube-prod-runtime/lib/kube.libsonnet';
local utils = import '../vendor/kube-prod-runtime/lib/utils.libsonnet';

local GANGWAY_IMAGE = 'gcr.io/heptio-images/gangway:v3.0.0';
local GANGWAY_PORT = 8080;
local GANGWAY_CONFIG_VOLUME_PATH = '/etc/gangway';
local GANGWAY_TLS_VOLUME_PATH = GANGWAY_CONFIG_VOLUME_PATH + '/tls';

{
  p:: '',

  sessionSecurityKey:: error 'sessionSecurityKey is undefined',

  base_domain:: '.cluster.local',

  app:: 'gangway',

  name:: $.p + $.app,

  domain:: $.name + $.base_domain,
  gangway_url:: 'https://' + $.domain,

  namespace:: 'gangway',

  config_path:: GANGWAY_CONFIG_VOLUME_PATH,

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
    usernameClaim: 'name',
    redirectURL: $.gangway_url + '/callback',
    clusterName: 'cluster-name',
    authorize_url: 'https://' + $.domain + '/auth',
    clientID: 'client-id',
    tokenURL: 'https://' + $.domain + '/token',
    scopes: [
      'openid',
      'email',
      'profile',
      'groups',
      'offline_access',
    ],
    serveTLS: true,
    certFile: GANGWAY_TLS_VOLUME_PATH + '/tls.crt',
    keyFile: GANGWAY_TLS_VOLUME_PATH + '/tls.key',
  },


  configMap: kube.ConfigMap($.name) + $.metadata {
    data+: {
      'gangway.yaml': std.manifestJsonEx($.config, '  '),
    },
  },

  secret: kube.Secret($.name) + $.metadata {
    data_+: {
      'session-security-key': $.sessionSecurityKey,
    },
  },

  deployment: kube.Deployment($.name) + $.metadata {
    local this = self,
    spec+: {
      replicas: 1,
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
            config: kube.ConfigMapVolume($.configMap),
            tls: {
              secret: {
                secretName: $.name + '-tls',
              },
            },
          },
          containers_+: {
            gangway: kube.Container($.app) {
              image: GANGWAY_IMAGE,
              command: [$.app],
              args: [
                '-config',
                GANGWAY_CONFIG_VOLUME_PATH + '/gangway.yaml',
              ],
              ports_+: {
                http: { containerPort: GANGWAY_PORT },
              },
              env_+: {
                GANGWAY_SESSION_SECURITY_KEY: kube.SecretKeyRef($.secret, 'session-security-key'),
                GANGWAY_PORT: GANGWAY_PORT,
              },
              readinessProbe: {
                httpGet: { path: '/', port: GANGWAY_PORT, scheme: 'HTTPS' },
                periodSeconds: 10,
              },
              livenessProbe: {
                httpGet: { path: '/', port: GANGWAY_PORT, scheme: 'HTTPS' },
                initialDelaySeconds: 20,
                periodSeconds: 10,
              },
              volumeMounts_+: {
                config: { mountPath: GANGWAY_CONFIG_VOLUME_PATH },
                tls: { mountPath: GANGWAY_TLS_VOLUME_PATH },
              },
            },
          },
        },
      },
    },
  },

  svc: kube.Service($.name) + $.metadata {
    target_pod: $.deployment.spec.template,
  },
}
