module "dns" {
  source = "../modules/google-dns"
  suffix = "${random_id.suffix.hex}"

  ca_crt_file = "${var.ca_crt_file}"
  ca_key_file = "${var.ca_key_file}"
}
