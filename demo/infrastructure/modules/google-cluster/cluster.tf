variable "suffix" {}

variable "zone" {}

resource "google_container_cluster" "cluster" {
  name = "cluster-${var.suffix}"
  zone = "${var.zone}"

  initial_node_count = 3

  # Setting an empty username and password explicitly disables basic auth
  master_auth {
    username = ""
    password = ""
  }

  node_config {
    oauth_scopes = [
      "https://www.googleapis.com/auth/compute",
      "https://www.googleapis.com/auth/devstorage.read_only",
      "https://www.googleapis.com/auth/logging.write",
      "https://www.googleapis.com/auth/monitoring",
    ]
  }
}

data "google_client_config" "default" {}

output "name" {
  value = "${google_container_cluster.cluster.name}"
}

output "project" {
  value = "${google_container_cluster.cluster.project}"
}

output "kubeconfig" {
  value = <<EOF
apiVersion: v1
kind: Config
clusters:
- name: ${google_container_cluster.cluster.name}
  cluster:
    certificate-authority-data :"${google_container_cluster.cluster.master_auth.0.cluster_ca_certificate}"
    server: "https://${google_container_cluster.cluster.endpoint}"
users:
- name: ${google_container_cluster.cluster.name}
  user:
    token: ${data.google_client_config.default.access_token}
contexts:
- context:
    cluster: ${google_container_cluster.cluster.name}
    user: ${google_container_cluster.cluster.name}
  name: ${google_container_cluster.cluster.name}
current-context: ${google_container_cluster.cluster.name}
EOF
}
