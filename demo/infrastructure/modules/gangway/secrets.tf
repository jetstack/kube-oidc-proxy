variable "length" {}

resource "random_id" "session_security_key" {
  byte_length = "16"
}

module "oauth2" {
  source = "../oauth2-secrets"
  length = "${var.length}"
}

resource "random_string" "client_secret" {
  length  = "${var.length}"
  special = false
}

output "config" {
  value = "${merge(module.oauth2.config,map("session_security_key",random_id.session_security_key.b64_std))}"
}
