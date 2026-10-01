resource "random_id" "pepper" {
  byte_length = 32
}

resource "google_secret_manager_secret" "pepper" {
  project   = var.project_id
  secret_id = "file-vault-hmac-pepper-${var.env}"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "pepper" {
  secret      = google_secret_manager_secret.pepper.id
  secret_data = random_id.pepper.b64_std
}
