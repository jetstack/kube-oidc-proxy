variable "length" {}

resource "random_string" "client_id" {
  length  = "${var.length}"
  special = false
}

resource "random_string" "client_secret" {
  length  = "${var.length}"
  special = false
}

output "config" {
  value = {
    client_id     = "${random_string.client_id.result}"
    client_secret = "${random_string.client_secret.result}"
  }
}
