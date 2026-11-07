variable "project_id" {
  type = string
}

variable "region" {
  type        = string
  description = "Firestore location_id. Must be a supported Firestore region or multi-region."
}

variable "env" {
  type = string
}

variable "delete_protection" {
  type        = bool
  default     = true
  description = "Block deleting the database (and keep it if Terraform destroys the resource)."
}
