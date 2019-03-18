local kube = import '../vendor/kube-prod-runtime/lib/kube.libsonnet';
local utils = import '../vendor/kube-prod-runtime/lib/utils.libsonnet';

local DEX_IMAGE = 'quay.io/dexidp/dex:v2.10.0';
local DEX_HTTPS_PORT = 5556;
local DEX_CONFIG_VOLUME_PATH = '/etc/dex/config';
local DEX_CONFIG_PATH = DEX_CONFIG_VOLUME_PATH + '/config.yaml';

local dexAPIGroup = 'dex.coreos.com';
local dexAPIVersion = 'v1';

local dexCRD(kind) = kube.CustomResourceDefinition('dex.coreos.com', 'v1beta1', kind) {
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

  base_domain:: 'dex.example.net',

  labels:: {
    metadata+: {
      labels+: {
        app: 'dex',
      },
    },
  },

  metadata:: $.labels {
    metadata+: {
      namespace: $.namespace,
    },
  },

  serviceAccount: kube.ServiceAccount($.p + 'dex') + $.metadata {
  },

  role: kube.Role($.p + 'dex') + $.metadata {
    rules: [
      {
        apiGroups: [''],
        resources: ['configmaps', 'secrets'],
        verbs: ['create', 'delete'],
      },
    ],
  },

  clusterRole: kube.ClusterRole($.p + 'dex') + $.metadata {
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

  roleBinding: kube.RoleBinding($.p + 'dex') + $.metadata {
    roleRef_: $.role,
    subjects_+: [$.serviceAccount],
  },

  clusterRoleBinding: kube.ClusterRoleBinding($.p + 'dex') + $.metadata {
    roleRef_: $.clusterRole,
    subjects_+: [$.serviceAccount],
  },

  disruptionBudget: kube.PodDisruptionBudget($.p + 'dex') + $.metadata {
    target_pod: $.deployment.spec.template,
    spec+: { maxUnavailable: 1 },
  },

  // ConfigMap for additional Java security properties
  config: kube.ConfigMap($.p + 'dex') + $.metadata {
    data+: {
      'config.yaml': std.manifestJsonEx({
        issuer: 'https://' + $.base_domain,
        storage: {
          type: 'kubernetes',
          config: {
            inCluster: true,
          },
        },
        web: {
          https: '0.0.0.0:5556',
          tlsCert: '/etc/dex/tls/tls.crt',
          tlsKey: '/etc/dex/tls/tls.key',
        },
        enablePasswordDB: true,
      }, '  '),
    },
  },
  deployment: kube.Deployment($.p + 'dex') + $.metadata {
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
          default_container: 'dex',
          volumes_+: {
            config: kube.ConfigMapVolume($.config),
          },
          securityContext: {
            fsGroup: 1001,
          },
          containers_+: {
            dex: kube.Container('dex') {
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
              },
              readinessProbe: {
                httpGet: { path: '/_cluster/health?local=true', port: 'db' },
                // don't allow rolling updates to kill containers until the cluster is green
                // ...meaning it's not allocating replicas or relocating any shards
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

  /* Dex creates them with no way of stopping it
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
        dexCRD('OfflineSessions'),
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
    )
    */

  svc: kube.Service($.p + 'dex') + $.metadata {
    target_pod: $.deployment.spec.template,
    spec+: {
      sessionAffinity: 'None',
    },
  },
}
