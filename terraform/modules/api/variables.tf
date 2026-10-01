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
  type        = string
  description = "Full Artifact Registry image tag for the Go API."
}

variable "bucket_name" {
  type = string
}

variable "pepper_secret_id" {
  type = string
}

variable "vpc_subnet_id" {
  type = string
}

variable "min_instances" {
  type    = number
  default = 0
}

variable "max_instances" {
  type    = number
  default = 10
}

variable "download_url_ttl_seconds" {
  type    = number
  default = 60
}

variable "share_ttl_days" {
  type    = number
  default = 7
}
