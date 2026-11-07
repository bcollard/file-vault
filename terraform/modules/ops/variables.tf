variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "env" {
  type = string
}

variable "alert_email" {
  type = string
}

variable "service_name" {
  type = string
}

variable "reuse_threshold" {
  type        = number
  default     = 10
  description = "Trigger alert when rejected reuse events exceed this count in 5 minutes."
}
