module "cluster" {
  source = "../modules/google-cluster"
  suffix = "${random_id.suffix.hex}"
  zone   = "${var.google_zone}"
}

output "cluster_kubeconfig" {
  value = "${module.cluster.kubeconfig}"
}
