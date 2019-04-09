provider "aws" {
  region = "eu-west-1"
}

module "cluster" {
  source = "../modules/amazon-cluster"
  suffix = "${random_id.suffix.hex}"
  #region = "${var.google_zone}"
}
