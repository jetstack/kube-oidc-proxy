provider "digitalocean" {}

variable "digitalocean_region" {
  default = "fra1"
}

variable "cluster_version" {
  default = "1.15.5-do.0"
}

module "cluster" {
  source = "../modules/digitalocean-cluster"
  suffix = "${random_id.suffix.hex}"

  cluster_version = "${var.cluster_version}"
  region          = "${var.digitalocean_region}"
}

variable "ca_crt_file" {}
variable "ca_key_file" {}
