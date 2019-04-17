(import './manifests/main.jsonnet') {
  base_domain: 'kubernetes.example.net',
  // dex_domain: 'dex.kubernetes.example.net',  // to be used on non dex hosting clusters
  cert_manager+: {
    letsencrypt_contact_email:: 'certificates@example.net',
  },
  dex+: {
    users: [
      $.dex.Password('admin@example.net', '$2y$10$i2.tSLkchjnpvnI73iSW/OPAVriV9BWbdfM6qemBM1buNRu81.ZG.'),  // plaintext: secure
    ],
    // This shows how to add dex connectors
    connectors: [
      $.dex.Connector('github', 'GitHub', 'github', {
        clientID: '0123',
        clientSecret: '4567',
        orgs: [{
          name: 'example-net',
        }],
      }),
    ],
  },
}
