variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "env" {
  type = string
}

variable "subnet_cidr" {
  type    = string
  default = "10.10.0.0/24"
}
