locals {
  config = {
    cert_manager = "${module.dns.config}"
    externaldns  = "${module.dns.config}"
  }

  secrets = {
    oidc_client_secret = "${module.secrets.oidc_client_secret}"
    gangway_key        = "${module.secrets.gangway_key}"
  }
}

output "config" {
  value = "${jsonencode(local.config)}"
}

# This fetches KUBECONFIG and grants full cluster admin access to the current user
output "kubeconfig_command" {
  value = "gcloud container clusters get-credentials ${module.cluster.name} --zone ${var.google_zone} --project ${module.cluster.project} && kubectl create clusterrolebinding cluster-admin-binding --clusterrole=cluster-admin --user=$(gcloud config get-value account) --dry-run -o yaml | kubectl apply -f -"
}

output "secrets" {
  value = "${jsonencode(local.secrets)}"
}
