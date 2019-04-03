variable "length" {}

resource "random_string" "oidc_client_secret" {
  length = "${var.length}"
  special = true
  override_special = "+/"
}

resource "random_string" "gangway_key" {
  length = "${var.length}"
  special = true
  override_special = "+/"
}

output "oidc_client_secret" {
  value = "${random_string.oidc_client_secret.result}="
}

output "gangway_key" {
  value = "${random_string.gangway_key.result}="
}
