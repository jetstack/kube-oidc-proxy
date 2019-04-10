variable "region" {
  default = "eu-west-1"
}

provider "aws" {
  region = "${var.region}"
}

module "cluster" {
  source = "../modules/amazon-cluster"
  suffix = "${random_id.suffix.hex}"
}
