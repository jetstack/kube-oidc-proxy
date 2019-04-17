variable "region" {
  default = "eu-west-1"
}

variable "cluster_version" {
  default = "1.12"
}

provider "aws" {
  region = "${var.region}"
}

module "cluster" {
  source  = "../modules/amazon-cluster"
  suffix  = "${random_id.suffix.hex}"

  cluster_version = "${var.cluster_version}"
}
