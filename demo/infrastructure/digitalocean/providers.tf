provider "digitalocean" {}

variable "digitalocean_region" {
  default = "fra1"
}

variable "cluster_version" {
  default = "1.12.8-do.1"
}

module "cluster" {
  source = "../modules/digitalocean-cluster"
  suffix = "${random_id.suffix.hex}"

  cluster_version = "${var.cluster_version}"
  region          = "${var.digitalocean_region}"
}
