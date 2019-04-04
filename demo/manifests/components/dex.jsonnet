local kube = import '../vendor/kube-prod-runtime/lib/kube.libsonnet';
local utils = import '../vendor/kube-prod-runtime/lib/utils.libsonnet';
local base32 = import 'base32.libsonnet';

local DEX_IMAGE = 'quay.io/dexidp/dex:v2.15.0';
local DEX_HTTPS_PORT = 5556;
local DEX_CONFIG_VOLUME_PATH = '/etc/dex/config';
local DEX_TLS_VOLUME_PATH = '/etc/dex/tls';
local DEX_CONFIG_PATH = DEX_CONFIG_VOLUME_PATH + '/config.yaml';

local dexAPIGroup = 'dex.coreos.com';
local dexAPIVersion = 'v1';

local dexCRD(kind) = kube.CustomResourceDefinition(dexAPIGroup, dexAPIVersion, kind) {
  spec+: {
    scope: 'Namespaced',
  },
};

// This a broken hash method dex is using
local fakeHashFNV(input) =
  local offset64 = std.stringChars('cbf29ce484222325');  //uint64 14695981039346656037
  local offset64Chars = std.map((function(o) std.parseHex(offset64[2 * o] + offset64[2 * o + 1])), std.range(0, 7));

  local bytes =
    if std.type(input) == 'string' then
      std.map(function(c) std.codepoint(c), input)
    else
      input;

  (bytes + offset64Chars);

// This hashes clientIDs and emails to metadata names for dex crds
local dexNameHash(s) = std.asciiLower(std.strReplace(base32.base32(fakeHashFNV(s)), '=', ''));


{
  // Create a entry in the password DB
  Password(email, hash):: kube._Object(dexAPIGroup + '/' + dexAPIVersion, 'Password', dexNameHash(email)) + {
    metadata+: {
      namespace: $.namespace,
    },
    email: email,
    hash: std.base64(hash),
    username: email,
  },

  // Create a client configuration for dex
  Client(name):: kube._Object(dexAPIGroup + '/' + dexAPIVersion, 'OAuth2Client', dexNameHash(name)) + {
    metadata+: {
      namespace: $.namespace,
    },
    id: name,
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

  config:: {
    issuer: 'https://' + $.domain,
    storage: {
      type: 'kubernetes',
      config: {
        inCluster: true,
      },
    },
    logger: {
      level: 'debug',
    },
    grpc: {
      addr: '127.0.0.1:5557',
      tlsCert: DEX_TLS_VOLUME_PATH + '/tls.crt',
      tlsKey: DEX_TLS_VOLUME_PATH + '/tls.key',
      tlsClientCA: DEX_TLS_VOLUME_PATH + '/tls.crt',
    },
    web: {
      https: '0.0.0.0:' + DEX_HTTPS_PORT,
      tlsCert: DEX_TLS_VOLUME_PATH + '/tls.crt',
      tlsKey: DEX_TLS_VOLUME_PATH + '/tls.key',
    },
    enablePasswordDB: true,
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
  configMap: kube.ConfigMap($.p + $.app) + $.metadata {
    data+: {
      'config.yaml': std.manifestJsonEx($.config, '  '),
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
          },
        },
        spec+: {
          serviceAccountName: $.serviceAccount.metadata.name,
          affinity: kube.PodZoneAntiAffinityAnnotation(this.spec.template),
          default_container: $.app,
          volumes_+: {
            config: kube.ConfigMapVolume($.configMap),
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
              },
              livenessProbe: self.readinessProbe {
                // elasticsearch_logging_discovery has a 5min timeout on cluster bootstrap
                initialDelaySeconds: 2 * 60,
                failureThreshold: 4,
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
