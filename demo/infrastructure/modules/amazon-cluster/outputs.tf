output "cluster_node_arn" {
  value = "${module.eks.worker_iam_role_arn}"
}

output "kubeconfig" {
  value = "${module.eks.kubeconfig}"
}
