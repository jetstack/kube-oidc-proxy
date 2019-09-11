variable "suffix" {}

variable "ca_crt_file" {}
variable "ca_key_file" {}

resource "google_service_account" "external_dns" {
  account_id   = "external-dns-${var.suffix}"
  display_name = "External DNS/Cert Manager service account for GKE cluster cluster-${var.suffix}"
}

# https://github.com/kubernetes-incubator/external-dns/blob/v0.4.0/docs/tutorials/gke.md#set-up-your-environment
resource "google_project_iam_member" "dns_admin" {
  role   = "roles/dns.admin"
  member = "serviceAccount:${google_service_account.external_dns.email}"
}

resource "google_service_account_key" "external_dns" {
  service_account_id = "${google_service_account.external_dns.account_id}"
}

data "local_file" "ca_crt" {
    filename = "${var.ca_crt_file}"
}

data "local_file" "ca_key" {
    filename = "${var.ca_key_file}"
}

output "config" {
  value = {
    service_account_credentials = "${base64decode(google_service_account_key.external_dns.private_key)}"

    project  = "${google_service_account.external_dns.project}"
    provider = "google"
    ca_crt   = "${data.local_file.ca_crt.content}"
    ca_key   = "${data.local_file.ca_key.content}"
  }
}
