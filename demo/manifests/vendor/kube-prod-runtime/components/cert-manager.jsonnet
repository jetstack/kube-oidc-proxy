/*
 * Bitnami Kubernetes Production Runtime - A collection of services that makes it
 * easy to run production workloads in Kubernetes.
 *
 * Copyright 2018-2019 Bitnami
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

local kube = import '../lib/kube.libsonnet';
local cert_manager_manifests = import './cert-manager/cert-manager.json';

{
  p:: '',
  metadata:: {
    metadata+: {
      namespace: 'kubeprod',
    },
  },
  letsencrypt_contact_email:: error 'Letsencrypt contact e-mail is undefined',

  // Letsencrypt environments
  letsencrypt_environments:: {
    prod: $.letsencryptProd.metadata.name,
    staging: $.letsencryptStaging.metadata.name,
  },
  // Letsencrypt environment (defaults to the production one)
  letsencrypt_environment:: 'prod',

  Issuer(name):: kube._Object('certmanager.k8s.io/v1alpha1', 'Issuer', name) {
  },

  ClusterIssuer(name):: kube._Object('certmanager.k8s.io/v1alpha1', 'ClusterIssuer', name) {
  },

  certCRD: kube.CustomResourceDefinition('certmanager.k8s.io', 'v1alpha1', 'Certificate') {
    spec+: { names+: { shortNames+: ['cert', 'certs'] } },
  },

  deploy: cert_manager_manifests,

  letsencryptStaging: $.ClusterIssuer($.p + 'letsencrypt-staging') {
    local this = self,
    spec+: {
      acme+: {
        server: 'https://acme-staging-v02.api.letsencrypt.org/directory',
        email: $.letsencrypt_contact_email,
        privateKeySecretRef: { name: this.metadata.name },
        http01: {},
      },
    },
  },

  letsencryptProd: $.letsencryptStaging {
    metadata+: { name: $.p + 'letsencrypt-prod' },
    spec+: {
      acme+: {
        server: 'https://acme-v02.api.letsencrypt.org/directory',
      },
    },
  },

  solvers+:: [],
}
