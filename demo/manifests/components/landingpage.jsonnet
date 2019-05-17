local kube = import '../vendor/kube-prod-runtime/lib/kube.libsonnet';
local utils = import '../vendor/kube-prod-runtime/lib/utils.libsonnet';

local IMAGE = 'nginx:1.15.12';
local WWW_VOLUME_PATH = '/usr/share/nginx/html';
local CONFIG_TOP_LEVEL_PATH = '/etc/nginx/nginx.conf';
local CONFIG_DEFAULT_PATH = '/etc/nginx/conf.d';
local HTTP_PORT = 80;

{
  p:: '',

  app:: 'landingpage',

  index:: false,

  name:: $.p + $.app,

  domain:: 'example.net',

  sslRedirectDomains:: ['redirect1.example.net', 'redirect2.example.net'],

  namespace:: 'auth',

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

  redirectStatement:: if std.length($.sslRedirectDomains) > 0 then
    'server {\n' +
    '    listen       80;\n' +
    '    server_name ' + std.join(' ', $.sslRedirectDomains) + ';\n' +
    '    return 301 https://$host$request_uri;\n' +
    '}\n'
  else
    '',

  configMap: kube.ConfigMap($.name) + $.metadata {
    data+: {
      'nginx.conf': |||
        user  nginx;
        worker_processes  1;

        error_log  /var/log/nginx/error.log warn;
        pid        /var/run/nginx.pid;


        events {
            worker_connections  1024;
        }


        http {
            include       /etc/nginx/mime.types;
            default_type  application/octet-stream;

            log_format  main  '$remote_addr - $remote_user [$time_local] "$request" '
                              '$status $body_bytes_sent "$http_referer" '
                              '"$http_user_agent" "$http_x_forwarded_for"';

            access_log  /var/log/nginx/access.log  main;

            sendfile        on;

            keepalive_timeout  65;

            server_names_hash_bucket_size 256;

            include /etc/nginx/conf.d/*.conf;
        }
      |||,
      'default.conf': |||
        server {
            listen       80;
            server_name  _;

            location / {
                root   /usr/share/nginx/html;
                index  index.html index.htm;
            }

            error_page   500 502 503 504  /50x.html;
            location = /50x.html {
                root   /usr/share/nginx/html;
            }
            if ($http_x_forwarded_proto = "http") {
                return 301 https://$host$request_uri;
            }
        }
      ||| + $.redirectStatement,
    },
  },

  Link(cloud, link, text):: std.manifestXmlJsonml([
    'div',
    { class: 'col s12 m4' },
    '\n  ',
    [
      'div',
      {},
      [
        'img',
        {
          width: 56,
          height: 56,
          alt: cloud,
          src: cloud + '.svg',
        },
      ],
    ],
    [
      'div',
      {},
      [
        'a',
        {
          class: 'btn-large waves-effect waves-light blue',
          href: link,
        },
        text,
      ],
    ],
    '\n',
  ]),

  content:: '',

  wwwConfigMap: if $.index then kube.ConfigMap($.name + '-www') + $.metadata {
    data+: {
      'index.html': std.strReplace(
        (importstr './landingpage/index.html'),
        '#CONTENT#',
        $.content,
      ),
      'amazon.svg': importstr './landingpage/amazon.svg',
      'google.svg': importstr './landingpage/google.svg',
      'digitalocean.svg': importstr './landingpage/digitalocean.svg',
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
          } + if $.index then { 'www/hash': std.md5(std.escapeStringJson($.wwwConfigMap)) } else {},
        },
        spec+: {
          affinity: kube.PodZoneAntiAffinityAnnotation(this.spec.template),
          default_container: $.app,
          volumes_+: {
            config: kube.ConfigMapVolume($.configMap),
          } + if $.index then { www: kube.ConfigMapVolume($.wwwConfigMap) } else {},
          containers_+: {
            landingpage: kube.Container($.app) {
              local container = self,
              image: IMAGE,
              args: [
                'nginx',
                '-g',
                'daemon off;',
                '-c',
                CONFIG_TOP_LEVEL_PATH,
              ],
              resources: {
                requests: { cpu: '10m', memory: '64Mi' },
              },
              ports_+: {
                http: { containerPort: HTTP_PORT },
              },
              volumeMounts_+: {
                config_top_level: { name: 'config', mountPath: CONFIG_TOP_LEVEL_PATH, subPath: 'nginx.conf' },
                config_default: { name: 'config', mountPath: CONFIG_DEFAULT_PATH + '/default.conf', subPath: 'default.conf' },
              } + if $.index then { www: { mountPath: WWW_VOLUME_PATH } } else {},
              readinessProbe: {
                httpGet: { path: '/', port: HTTP_PORT },
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


  ingress: if std.length($.sslRedirectDomains) > 0 || $.index then kube.Ingress($.name) + $.metadata {
    local hosts = $.sslRedirectDomains + if $.index then [$.domain] else [],
    metadata+: {
      annotations+: {
        'kubernetes.io/ingress.class': 'contour',
      },
    },
    spec+: {
      rules+: std.map(
        (function(h) {
           host: h,
           http: {
             paths: [
               { path: '/', backend: $.svc.name_port },
             ],
           },
         }), hosts
      ),
    } + if $.index && $.certificate != null then {
      tls: [{
        hosts: [$.domain],
        secretName: $.certificate.spec.secretName,
      }],
    } else {},
  },


  svc: kube.Service($.name) + $.metadata {
    target_pod: $.deployment.spec.template,
    spec+: {
      sessionAffinity: 'None',
    },
  },
}
