locals {
  config = {
    cert-manager = "${module.dns.config}"
    externaldns  = "${module.dns.config}"
  }
}

output "config" {
  value = "${jsonencode(local.config)}"
}
