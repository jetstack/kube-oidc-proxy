#locals {
#  config = {
#    cert_manager = "${module.dns.config}"
#    externaldns  = "${module.dns.config}"
#    gangway      = "${module.gangway.config}"
#  }
#}

output "config" {
  #value = "${jsonencode(local.config)}"
  value = ""
}

output "kubeconfig_command" {
  value = "${module.cluster.config_map_aws_auth} | kubectl apply -f -"
}

output "kubeconfig" {
  value = "${module.cluster.kubeconfig}"
}
