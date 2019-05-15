variable "region" {}
variable "suffix" {}
variable "cluster_version" {}

locals {
  cluster_name = "cluster-${var.suffix}"
}

resource "digitalocean_kubernetes_cluster" "cluster" {
  name    = "${local.cluster_name}"
  version = "${var.cluster_version}"
  region  = "${var.region}"

  node_pool {
    name       = "default-pool"
    size       = "s-2vcpu-2gb"
    node_count = 3
  }
}
