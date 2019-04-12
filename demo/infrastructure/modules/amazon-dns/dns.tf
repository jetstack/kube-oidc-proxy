variable "suffix" {}
variable "region" {}

resource "aws_iam_user" "dns" {
  name = "cluster-dns-${var.suffix}"
  path = "/"
}

resource "aws_iam_access_key" "dns" {
  user = "${aws_iam_user.dns.name}"
}

resource "aws_iam_user_policy" "dns" {
  name = "cluster-dns-${var.suffix}"
  user = "${aws_iam_user.dns.name}"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "route53:GetHostedZone",
        "route53:ListHostedZones",
        "route53:ListHostedZonesByName",
        "route53:GetHostedZoneCount",
        "route53:ChangeResourceRecordSets",
        "route53:ListResourceRecordSets",
        "route53:GetChange"
      ],
      "Resource": "*"
    }
  ]
}
EOF
}


output "config" {
  value = {
    service_account_credentials = "${aws_iam_access_key.dns.id}"
    provider                    = "route53"
    region                      = "${var.region}"
  }
}
