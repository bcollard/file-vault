variable "project_id" {
  type = string
}

variable "region" {
  type    = string
  default = "europe-west9"
}

variable "bucket_name" {
  type = string
}

variable "object_ttl_days" {
  type    = number
  default = 7
}

variable "share_ttl_days" {
  type    = number
  default = 7
}

variable "domain" {
  type        = string
  description = "Public hostname served by your external LB (e.g. file-vault.example.com)."
}

variable "api_image" {
  type = string
}

variable "web_image" {
  type = string
}

variable "sweeper_image" {
  type = string
}

variable "alert_email" {
  type = string
}

variable "force_destroy_bucket" {
  type    = bool
  default = false
}
