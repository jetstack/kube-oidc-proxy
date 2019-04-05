local kube = import '../vendor/kube-prod-runtime/lib/kube.libsonnet';
local utils = import '../vendor/kube-prod-runtime/lib/utils.libsonnet';

local GANGWAY_IMAGE = 'gcr.io/heptio-images/gangway:v3.0.0';
local GANGWAY_TLS_VOLUME_PATH = '/etc/dex/tls';

{
  p:: '',

  sessionSecurityKey:: error 'sessionSecurityKey is undefined',

  base_domain:: 'cluster.local',

  app:: 'gangway',
  domain:: $.app + '.' + $.base_domain,
  gangway_url:: 'https://' + $.domain,

  namespace:: 'gangway',


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
    clusterName: 'cluster-name',
    serveTLS: true,
    certFile: GANGWAY_TLS_VOLUME_PATH + '/tls.crt',
    keyFile: GANGWAY_TLS_VOLUME_PATH + '/tls.key',
  },


  configMap: kube.ConfigMap($.p + $.app) + $.metadata {
    data+: {
      'gangway.yaml': std.manifestJsonEx($.config, '  '),
    },
  },

  secret: kube.Secret($.p + $.app) + $.metadata {
    data_+: {
      'session-security-key': $.sessionSecurityKey,
    },
  },

  deployment: kube.Deployment($.p + $.app) + $.metadata {
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
                secretName: $.app + '-tls',
              },
            },
          },
          containers_+: {
            gangway: kube.Container($.app) {
              image: GANGWAY_IMAGE,
              command: [$.app],
              args: [
                '-config',
                '/config/gangway.yaml',
              ],
              ports_+: {
                http: { containerPort: 8080 },
              },
              env_+: {
                GANGWAY_SESSION_SECURITY_KEY: kube.SecretKeyRef($.secret, 'session-security-key'),
                GANGWAY_PORT: '8080',
              },
              readinessProbe: {
                httpGet: { path: '/', port: 8080, scheme: 'HTTPS' },
                periodSeconds: 10,
              },
              livenessProbe: {
                httpGet: { path: '/', port: 8080, scheme: 'HTTPS' },
                initialDelaySeconds: 20,
                periodSeconds: 10,
              },
              volumeMounts_+: {
                config: { mountPath: '/config' },
                tls: { mountPath: GANGWAY_TLS_VOLUME_PATH },
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
