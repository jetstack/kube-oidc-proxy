module "dns" {
  source = "../modules/amazon-dns"
  suffix = "${random_id.suffix.hex}"
  region = "${var.region}"
}
