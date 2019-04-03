local kube = import '../vendor/kube-prod-runtime/lib/kube.libsonnet';
local utils = import '../vendor/kube-prod-runtime/lib/utils.libsonnet';

local DEX_IMAGE = 'quay.io/dexidp/dex:v2.10.0';
local DEX_HTTPS_PORT = 5556;
local DEX_CONFIG_VOLUME_PATH = '/etc/dex/config';
local DEX_TLS_VOLUME_PATH = '/etc/dex/tls';
local DEX_CONFIG_PATH = DEX_CONFIG_VOLUME_PATH + '/config.yaml';

local dexAPIGroup = 'dex.coreos.com';
local dexAPIVersion = 'v1';

local dexCRD(kind) = kube.CustomResourceDefinition(dexAPIGroup, dexAPIVersion, kind) {
  spec+: {
    scope: 'Cluster',
  },
};


{
  // Create a entry in the password DB
  Password(name, email, hash):: kube._Object(dexAPIGroup + '/' + dexAPIVersion, 'Password', name) + {
    metadata+: {
      namespace: $.namespace,
    },
    email: email,
    hash: hash,
    username: name,
  },
  p:: '',

  namespace:: 'auth',

  base_domain:: 'example.net',

  app:: 'dex',
  domain:: $.app + '.' + $.base_domain,

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

  serviceAccount: kube.ServiceAccount($.p + $.app) + $.metadata {
  },

  role: kube.Role($.p + $.app) + $.metadata {
    rules: [
      {
        apiGroups: [''],
        resources: ['configmaps', 'secrets'],
        verbs: ['create', 'delete'],
      },
    ],
  },

  clusterRole: kube.ClusterRole($.p + $.app) + $.metadata {
    rules: [
      {
        apiGroups: ['dex.coreos.com'],  // API group created by dex
        resources: ['*'],
        verbs: ['*'],
      },
      {
        apiGroups: ['apiextensions.k8s.io'],
        resources: ['customresourcedefinitions'],
        verbs: ['create'],  // To manage its own resources, dex must be able to create customresourcedefinitions
      },
    ],
  },

  roleBinding: kube.RoleBinding($.p + $.app) + $.metadata {
    roleRef_: $.role,
    subjects_+: [$.serviceAccount],
  },

  clusterRoleBinding: kube.ClusterRoleBinding($.p + $.app) + $.metadata {
    roleRef_: $.clusterRole,
    subjects_+: [$.serviceAccount],
  },

  disruptionBudget: kube.PodDisruptionBudget($.p + $.app) + $.metadata {
    target_pod: $.deployment.spec.template,
    spec+: { maxUnavailable: 1 },
  },

  // ConfigMap for additional Java security properties
  config: kube.ConfigMap($.p + $.app) + $.metadata {
    data+: {
      'config.yaml': std.manifestJsonEx({
        issuer: 'https://' + $.domain,
        storage: {
          type: 'kubernetes',
          config: {
            inCluster: true,
          },
        },
        web: {
          https: '0.0.0.0:' + DEX_HTTPS_PORT,
          tlsCert: DEX_TLS_VOLUME_PATH + '/tls.crt',
          tlsKey: DEX_TLS_VOLUME_PATH + '/tls.key',
        },
        enablePasswordDB: true,
      }, '  '),
    },
  },
  deployment: kube.Deployment($.p + $.app) + $.metadata {
    local this = self,
    spec+: {
      replicas: 1,
      template+: {
        metadata+: {
          annotations+: {
            'config/hash': std.md5(std.escapeStringJson($.config)),
          },
        },
        spec+: {
          serviceAccountName: $.serviceAccount.metadata.name,
          affinity: kube.PodZoneAntiAffinityAnnotation(this.spec.template),
          default_container: $.app,
          volumes_+: {
            config: kube.ConfigMapVolume($.config),
            tls: {
              secret: {
                secretName: $.p + 'dex-tls',
              },
            },
          },
          securityContext: {
            fsGroup: 1001,
          },
          containers_+: {
            dex: kube.Container($.app) {
              local container = self,
              image: DEX_IMAGE,
              command: ['/usr/local/bin/dex', 'serve', DEX_CONFIG_PATH],
              // This can massively vary depending on the logging volume
              securityContext: {
                runAsUser: 1001,
              },
              resources: {
                requests: { cpu: '100m', memory: '512Mi' },
              },
              ports_+: {
                https: { containerPort: DEX_HTTPS_PORT },
              },
              volumeMounts_+: {
                config: { mountPath: DEX_CONFIG_VOLUME_PATH },
                tls: { mountPath: DEX_TLS_VOLUME_PATH },
              },
              readinessProbe: {
                httpGet: { path: '/.well-known/openid-configuration', port: DEX_HTTPS_PORT, scheme: 'HTTPS' },
                initialDelaySeconds: 120,
                periodSeconds: 30,
                failureThreshold: 4,
                successThreshold: 2,  // Minimum consecutive successes for the probe to be considered successful after having failed.
              },
              livenessProbe: self.readinessProbe {
                // elasticsearch_logging_discovery has a 5min timeout on cluster bootstrap
                initialDelaySeconds: 5 * 60,
                successThreshold: 1,  // Minimum consecutive successes for the probe to be considered successful after having failed.
              },
            },
          },
        },
      },
    },
  },

  crds: std.map(
    (function(o) $.labels + o),
    [
      dexCRD('AuthCode'),
      dexCRD('AuthRequest'),
      dexCRD('Connector'),
      dexCRD('OfflineSessions') + {
        metadata+: {
          name: 'offlinesessionses.dex.coreos.com',
        },
        spec+: {
          names+: {
            plural: 'offlinesessionses',
          },
        },
      },
      dexCRD('OAuth2Client'),
      dexCRD('Password'),
      dexCRD('RefreshToken'),
      dexCRD('SigningKey') + {
        metadata+: {
          name: 'signingkeies.dex.coreos.com',
        },
        spec+: {
          names+: {
            plural: 'signingkeies',
          },
        },
      },
    ]
  ),

  svc: kube.Service($.p + $.app) + $.metadata {
    target_pod: $.deployment.spec.template,
    spec+: {
      sessionAffinity: 'None',
    },
  },
}
