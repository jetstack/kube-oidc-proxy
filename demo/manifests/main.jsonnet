local kube = import './vendor/kube-prod-runtime/lib/kube.libsonnet';

local cert_manager = import './components/cert-manager.jsonnet';
local externaldns = import './vendor/kube-prod-runtime/components/externaldns.jsonnet';

local contour = import './components/contour.jsonnet';
local dex = import './components/dex.jsonnet';
local gangway = import './components/gangway.jsonnet';
local kube_oidc_proxy = import './components/kube-oidc-proxy.jsonnet';

local config = import './config.json';

local namespace = 'auth';

local IngressRouteTLSPassthrough(namespace, name, domain, serviceName, servicePort) = contour.IngressRoute(
  namespace,
  name,
) {
  spec+: {
    virtualhost: {
      fqdn: domain,
      tls: {
        passthrough: true,
      },
    },
    tcpproxy: {
      services: [
        {
          name: serviceName,
          port: servicePort,
        },
      ],
    },
    routes: [
      {
        match: '/',
        services: [
          {
            name: 'fake',
            port: 6666,
          },
        ],
      },
    ],
  },
};

{
  config:: config,

  base_domain:: error 'base_domain is undefined',
  letsencrypt_contact_email:: error 'letsencrypt_contact_email is undefined',

  namespace: kube.Namespace(namespace),

  cert_manager: cert_manager {
    google_secret: kube.Secret($.cert_manager.p + 'clouddns-google-credentials') + $.cert_manager.metadata {
      data_+: {
        'credentials.json': $.config.cert_manager.service_account_credentials,
      },
    },

    metadata:: {
      metadata+: {
        namespace: 'kube-system',
      },
    },
    letsencrypt_contact_email:: $.letsencrypt_contact_email,
    letsencrypt_environment:: 'prod',

    letsencryptStaging+: {
      spec+: {
        acme+: {
          http01: null,
          dns01: {
            providers: [{
              name: 'clouddns',
              clouddns: {
                project: $.config.cert_manager.project,
                serviceAccountSecretRef: {
                  name: $.cert_manager.google_secret.metadata.name,
                  key: 'credentials.json',
                },
              },
            }],
          },
        },
      },
    },
  },

  externaldns: externaldns {
    metadata:: {
      metadata+: {
        namespace: 'kube-system',
      },
    },

    gcreds: kube.Secret($.externaldns.p + 'externaldns-google-credentials') + $.externaldns.metadata {
      data_+: {
        'credentials.json': $.config.externaldns.service_account_credentials,
      },
    },

    deploy+: {
      ownerId: $.base_domain,
      spec+: {
        template+: {
          spec+: {
            volumes_+: {
              gcreds: kube.SecretVolume($.externaldns.gcreds),
            },
            containers_+: {
              edns+: {
                args_+: {
                  provider: 'google',
                  'google-project': $.config.externaldns.project,
                },
                env_+: {
                  GOOGLE_APPLICATION_CREDENTIALS: '/google/credentials.json',
                },
                volumeMounts_+: {
                  gcreds: { mountPath: '/google', readOnly: true },
                },
              },
            },
          },
        },
      },
    },
  },

  contour: contour {
    base_domain:: $.base_domain,

    metadata:: {
      metadata+: {
        namespace: namespace,
      },
    },

    svc+: {
      metadata+: {
        annotations+: {
          // this add a final dot to the domain name and joins the list
          'external-dns.alpha.kubernetes.io/hostname': std.join(',', std.map(
            (function(o) o + '.'),
            [$.dex.domain, $.gangway.domain, $.kube_oidc_proxy.domain],
          )),
        },
      },
    },
  },

  dex: dex {
    local this = self,
    namespace:: namespace,
    base_domain:: $.base_domain,
    client_secret:: $.config.gangway.client_secret,

    certificate: cert_manager.Certificate(
      namespace,
      this.app,
      $.cert_manager.letsencryptProd,
      [this.domain]
    ),
    ingressRoute: IngressRouteTLSPassthrough(namespace, this.app, this.domain, this.app, 5556),
  },

  dexPasswordChristian: dex.Password('simon@swine.de', '$2y$10$i2.tSLkchjnpvnI73iSW/OPAVriV9BWbdfM6qemBM1buNRu81.ZG.'),  // plaintext: secure

  gangway: gangway {
    local this = self,
    base_domain:: $.base_domain,
    secret_key:: $.config.gangway.session_security_key,
    client_secret:: $.config.gangway.client_secret,
    metadata:: {
      metadata+: {
        namespace: namespace,
      },
    },

    certificate: cert_manager.Certificate(
      namespace,
      this.app,
      $.cert_manager.letsencryptProd,
      [this.domain]
    ),
    ingressRoute: IngressRouteTLSPassthrough(namespace, this.app, this.domain, this.app, 8080),

    sessionSecurityKey: $.config.gangway.session_security_key,

    config+: {
      authorizeURL: 'https://' + $.dex.domain + '/auth',
      tokenURL: 'https://' + $.dex.domain + '/token',
      apiServerURL: 'https://' + $.kube_oidc_proxy.domain,
      clientID: $.config.gangway.client_id,
      clientSecret: $.config.gangway.client_secret,
    },

    dexClient: dex.Client(this.config.clientID) {
      secret: this.config.clientSecret,
      redirectURIs: [
        this.config.redirectURL,
      ],
    },
  },

  kube_oidc_proxy: kube_oidc_proxy {
    local this = self,
    base_domain:: $.base_domain,
    metadata:: {
      metadata+: {
        namespace: namespace,
      },
    },

    config+: {
      oidc+: {
        issuerURL: 'https://' + $.dex.domain,
        clientID: $.config.gangway.client_id,
      },
    },

    client_secret:: $.config.gangway.client_secret,

    certificate: cert_manager.Certificate(
      namespace,
      this.app,
      $.cert_manager.letsencryptProd,
      [this.domain]
    ),
    ingressRoute: IngressRouteTLSPassthrough(namespace, this.app, this.domain, this.app, 443),
  },
}
