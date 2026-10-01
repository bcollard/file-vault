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
