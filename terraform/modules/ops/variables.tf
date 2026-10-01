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

variable "alert_email" {
  type = string
}

variable "api_service" {
  type = string
}

variable "sweeper_image" {
  type        = string
  description = "Image for the sweeper job. Defaults to the cloudrun hello image until you push the real sweeper."
  default     = "gcr.io/cloudrun/hello"
}

variable "sweep_cron" {
  type    = string
  default = "0 * * * *"
}

variable "reuse_threshold" {
  type        = number
  default     = 10
  description = "Trigger alert when rejected reuse events exceed this count in 5 minutes."
}
