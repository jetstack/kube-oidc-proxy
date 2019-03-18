module "dns" {
  source = "../modules/google-dns"
  suffix = "${random_id.suffix.hex}"
}
