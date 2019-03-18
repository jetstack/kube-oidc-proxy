locals {
  config = {
    cert_manager = "${module.dns.config}"
    externaldns  = "${module.dns.config}"
  }
}

output "config" {
  value = "${jsonencode(local.config)}"
}
