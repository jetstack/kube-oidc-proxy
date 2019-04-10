data "aws_availability_zones" "available" {}

resource "aws_vpc" "cluster" {
  cidr_block = "10.0.0.0/16"

  tags = "${
    map(
     "Name", "cluster-${var.suffix}",
     "kubernetes.io/cluster/cluster-${var.suffix}", "shared",
    )
  }"
}

resource "aws_subnet" "cluster" {
  count = 2

  availability_zone = "${data.aws_availability_zones.available.names[count.index]}"
  cidr_block        = "10.0.${count.index}.0/24"
  vpc_id            = "${aws_vpc.cluster.id}"

  tags = "${
    map(
     "Name", "cluster-${var.suffix}",
     "kubernetes.io/cluster/cluster-${var.suffix}", "shared",
    )
  }"
}

resource "aws_internet_gateway" "cluster" {
  vpc_id = "${aws_vpc.cluster.id}"

  tags {
    Name = "cluster-${var.suffix}"
  }
}

resource "aws_route_table" "cluster" {
  vpc_id = "${aws_vpc.cluster.id}"

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = "${aws_internet_gateway.cluster.id}"
  }
}

resource "aws_route_table_association" "cluster" {
  count = 2

  subnet_id      = "${aws_subnet.cluster.*.id[count.index]}"
  route_table_id = "${aws_route_table.cluster.id}"
}
