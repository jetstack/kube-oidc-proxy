variable "aws_region" {
  default = "eu-west-1"
}

provider "aws" {
  region = "${var.aws_region}"
}

module "cluster" {
  source = "../modules/amazon-cluster"
  suffix = "${random_id.suffix.hex}"
}
