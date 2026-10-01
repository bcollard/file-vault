variable "project_id" {
  type        = string
  description = "GCP project ID."
}

variable "region" {
  type        = string
  default     = "europe-west9"
  description = "Default region for regional resources. Firestore must support this location."
}

variable "bucket_name" {
  type        = string
  description = "Globally-unique GCS bucket name for share objects."
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
  type        = string
  default     = "gcr.io/cloudrun/hello"
  description = "Replace with your Artifact Registry tag once the Go API is built."
}

variable "web_image" {
  type        = string
  default     = "gcr.io/cloudrun/hello"
  description = "Replace with your Artifact Registry tag once the web UI is built."
}

variable "sweeper_image" {
  type    = string
  default = "gcr.io/cloudrun/hello"
}

variable "alert_email" {
  type        = string
  description = "Email for the Monitoring notification channel."
}

variable "force_destroy_bucket" {
  type    = bool
  default = true
}
