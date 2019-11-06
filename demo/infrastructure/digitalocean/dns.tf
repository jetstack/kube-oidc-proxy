data "external" "cert_manager" {
  program = ["jq", ".cert_manager", "../../manifests/google-config.json"]
  query   = {}
}

data "external" "externaldns" {
  program = ["jq", ".externaldns", "../../manifests/google-config.json"]
  query   = {}
}

module "ca" {
  source = "../modules/ca"

  ca_crt_file = "${var.ca_crt_file}"
  ca_key_file = "${var.ca_key_file}"
}
