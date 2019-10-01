variable "aws_region" {
  default = "eu-west-1"
}

variable "cloud" {
  default = "amazon"
}

variable "cluster_version" {
  default = "1.14"
}

provider "aws" {
  region = "${var.aws_region}"
}

module "cluster" {
  source  = "../modules/amazon-cluster"
  suffix  = "${random_id.suffix.hex}"

  cluster_version = "${var.cluster_version}"
}
