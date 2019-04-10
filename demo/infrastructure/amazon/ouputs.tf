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
  value = "eksctl utils write-kubeconfig --name=cluster-${random_id.suffix.hex} --kubeconfig=$KUBECONFIG --set-kubeconfig-context=true --region=${var.aws_region} && cat infrastructure/amazon/aws-auth.yaml | sed -e \"s~{{ROLE_ARN}}~${module.cluster.cluster_node_arn}~g\" | kubectl apply -f -"
}

output "kubeconfig" {
  value = "${module.cluster.kubeconfig}"
}
