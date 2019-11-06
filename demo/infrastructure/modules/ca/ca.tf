variable "ca_crt_file" {}
variable "ca_key_file" {}

data "local_file" "crt_file" {
    filename = "${var.ca_crt_file}"
}

data "local_file" "key_file" {
    filename = "${var.ca_key_file}"
}


output "crt" {
  value = "${data.local_file.crt_file.content}"
}

output "key" {
  value = "${data.local_file.key_file.content}"
}
