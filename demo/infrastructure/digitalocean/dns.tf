data "external" "cert_manager" {
  program = ["jq", ".cert_manager", "../../manifests/google-config.json"]
  query   = {}
}

data "external" "externaldns" {
  program = ["jq", ".externaldns", "../../manifests/google-config.json"]
  query   = {}
}
