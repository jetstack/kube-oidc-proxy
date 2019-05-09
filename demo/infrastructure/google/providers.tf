variable "google_region" {
  default = "europe-west1"
}

variable "google_zone" {
  default = "europe-west1-d"
}

variable "google_project" {}

variable "ca_crt_file" {}
variable "ca_key_file" {}

provider "google" {
  region      = "${var.google_region}"
  credentials = "${file("~/.config/gcloud/terraform-admin.json")}"
  project     = "${var.google_project}"
  version     = "~> 2.0"
}
