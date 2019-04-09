variable "suffix" {}

data "aws_region" "current" {}

resource "aws_eks_cluster" "cluster" {
  name     = "cluster-${var.suffix}"
  role_arn = "${aws_iam_role.cluster.arn}"

  vpc_config {
    security_group_ids = ["${aws_security_group.cluster.id}"]
    subnet_ids         = ["${aws_subnet.cluster.*.id}"]
  }

  depends_on = [
    "aws_iam_role_policy_attachment.cluster-AmazonEKSClusterPolicy",
    "aws_iam_role_policy_attachment.cluster-AmazonEKSServicePolicy",
  ]
}

data "aws_ami" "eks-worker" {
  filter {
    name   = "name"
    values = ["amazon-eks-node-${aws_eks_cluster.cluster.version}-v*"]
  }

  most_recent = true
  owners      = ["602401143452"] # Amazon EKS AMI Account ID
}

# EKS currently documents this required userdata for EKS worker nodes to
# properly configure Kubernetes applications on the EC2 instance.
# We utilize a Terraform local here to simplify Base64 encoding this
# information into the AutoScaling Launch Configuration.
# More information: https://docs.aws.amazon.com/eks/latest/userguide/launch-workers.html
locals {
  cluster-node-userdata = <<USERDATA
#!/bin/bash
set -o xtrace
/etc/eks/bootstrap.sh --apiserver-endpoint '${aws_eks_cluster.cluster.endpoint}' --b64-cluster-ca '${aws_eks_cluster.cluster.certificate_authority.0.data}' 'cluster-${var.suffix}'
USERDATA
}

resource "aws_launch_configuration" "cluster" {
  associate_public_ip_address = true
  iam_instance_profile        = "${aws_iam_instance_profile.cluster-node.name}"
  image_id                    = "${data.aws_ami.eks-worker.id}"
  instance_type               = "m4.large"
  name_prefix                 = "cluster-${var.suffix}"
  security_groups             = ["${aws_security_group.cluster-node.id}"]
  user_data_base64            = "${base64encode(local.cluster-node-userdata)}"

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_autoscaling_group" "cluster" {
  desired_capacity     = 2
  launch_configuration = "${aws_launch_configuration.cluster.id}"
  max_size             = 2
  min_size             = 1
  name                 = "terraform-eks-cluster"
  vpc_zone_identifier  = ["${aws_subnet.cluster.*.id}"]

  tag {
    key                 = "Name"
    value               = "cluster-${var.suffix}"
    propagate_at_launch = true
  }

  tag {
    key                 = "kubernetes.io/cluster/cluster-${var.suffix}"
    value               = "owned"
    propagate_at_launch = true
  }
}
