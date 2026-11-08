# Deployment-specific values go in terraform.tfvars (gitignored). See
# terraform.tfvars.example. CI writes the same file from the TFVARS secret.

variable "project_id" {
  type        = string
  description = "GCP project ID."
}

variable "region" {
  type        = string
  default     = "europe-west9"
  description = "Region for Cloud Run, the bucket, KMS and Firestore."
}

variable "domain" {
  type        = string
  description = "Public hostname served by your load balancer, e.g. file-vault.example.com."
}

variable "bucket_name" {
  type        = string
  description = "Globally unique GCS bucket name for encrypted objects."
}

variable "alert_email" {
  type        = string
  sensitive   = true
  description = "Email for the Monitoring notification channel. Sensitive so plans and public CI logs redact it."
}

variable "image" {
  type        = string
  description = "Container image. The deploy workflow passes the tag it just built (-var image=...)."
}

variable "share_ttl_days" {
  type    = number
  default = 7
}

variable "max_upload_mb" {
  type    = number
  default = 1024
}

variable "min_instances" {
  type    = number
  default = 0
}

variable "max_instances" {
  type    = number
  default = 10
}

variable "force_destroy_bucket" {
  type    = bool
  default = false
}
