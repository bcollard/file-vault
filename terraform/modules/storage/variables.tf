variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "env" {
  type = string
}

variable "bucket_name" {
  type = string
}

variable "object_ttl_days" {
  type    = number
  default = 7
}

variable "force_destroy" {
  type        = bool
  default     = false
  description = "Allow terraform destroy to delete a non-empty bucket. Set true in dev only."
}

variable "cors_origins" {
  type        = list(string)
  description = "Origins allowed to upload and download directly, e.g. [\"https://file-vault.example.com\"]."
}
