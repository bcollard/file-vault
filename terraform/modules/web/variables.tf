variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "env" {
  type = string
}

variable "image" {
  type = string
}

variable "api_url" {
  type        = string
  description = "Cloud Run *.run.app URL of the API service."
}

variable "api_service" {
  type = string
}

variable "vpc_subnet_id" {
  type = string
}

variable "public_domain" {
  type        = string
  description = "Public hostname users see (e.g. file-vault.example.com). Used to render share/admin URLs."
}

variable "min_instances" {
  type    = number
  default = 0
}

variable "max_instances" {
  type    = number
  default = 10
}
