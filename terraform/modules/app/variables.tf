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
  description = "Full Artifact Registry image tag (UI embedded in the Go binary)."
}

variable "bucket_name" {
  type = string
}

variable "pepper_secret_id" {
  type = string
}

variable "public_domain" {
  type        = string
  description = "Hostname users load the UI from, e.g. file-vault.example.com. Upload sessions are bound to https://<this>."
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

variable "audit_retention_days" {
  type        = number
  default     = 30
  description = "Firestore TTL on audit events (they hold client IPs)."
}

variable "max_upload_mb" {
  type        = number
  default     = 1024
  description = "Largest ciphertext accepted. Downloads are decrypted in browser memory, so keep this moderate."
}

variable "trusted_proxy_hops" {
  type        = number
  default     = 2
  description = "X-Forwarded-For entries appended by trusted proxies. 2 = Google external Application LB."
}

variable "firestore_database" {
  type = string
}
