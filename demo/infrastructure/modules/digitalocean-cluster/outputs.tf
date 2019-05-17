output "kubeconfig" {
  value = "${digitalocean_kubernetes_cluster.cluster.kube_config.0.raw_config}"
}
