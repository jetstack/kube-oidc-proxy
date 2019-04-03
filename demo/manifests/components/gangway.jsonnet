local kube = import '../vendor/kube-prod-runtime/lib/kube.libsonnet';
local utils = import '../vendor/kube-prod-runtime/lib/utils.libsonnet';

local GANGWAY_IMAGE = 'gcr.io/heptio-images/gangway:v3.0.0';

{
  p:: '',

  base_domain:: 'cluster.local',

  app:: 'gangway',
  domain:: $.app + '.' + $.base_domain,

  cluster_name:: 'mycluster',

  namespace:: 'gangway',

  gangway_url:: 'https://' + $.domain,
  kubernetes_url:: 'https://kubernetes-api.' + $.base_domain,

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
    apiServerURL: $.kubernetes_url,
    redirectURL: $.gangway_url + '/callback',
    clusterName: $.cluster_name,
  },


  configMap: kube.ConfigMap($.p + $.app) + $.metadata {
    data+: {
      'gangway.yaml': std.manifestJsonEx($.config, '  '),
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
          },
        },
        spec+: {
          affinity: kube.PodZoneAntiAffinityAnnotation(this.spec.template),
          default_container: $.app,
          volumes_+: {
            config: kube.ConfigMapVolume($.configMap),
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
                GANGWAY_PORT: '8080',
              },
              readinessProbe: {
                httpGet: { path: '/', port: 8080 },
                periodSeconds: 10,
              },
              livenessProbe: {
                httpGet: { path: '/', port: 8080 },
                initialDelaySeconds: 20,
                periodSeconds: 10,
              },
              volumeMounts_+: {
                config: { mountPath: '/config' },
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
