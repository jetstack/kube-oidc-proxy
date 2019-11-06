module "dns" {
  source = "../modules/google-dns"
  suffix = "${random_id.suffix.hex}"
}

module "ca" {
  source  = "../modules/ca"

  ca_crt_file = "${var.ca_crt_file}"
  ca_key_file = "${var.ca_key_file}"
}
