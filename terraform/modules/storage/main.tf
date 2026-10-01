data "google_project" "this" {
  project_id = var.project_id
}

resource "google_kms_key_ring" "this" {
  project  = var.project_id
  name     = "file-vault-${var.env}"
  location = var.region
}

resource "google_kms_crypto_key" "bucket" {
  name            = "file-vault-bucket-${var.env}"
  key_ring        = google_kms_key_ring.this.id
  purpose         = "ENCRYPT_DECRYPT"
  rotation_period = "7776000s" # 90 days
}

resource "google_kms_crypto_key_iam_member" "gcs_sa_encrypter" {
  crypto_key_id = google_kms_crypto_key.bucket.id
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:service-${data.google_project.this.number}@gs-project-accounts.iam.gserviceaccount.com"
}

resource "google_storage_bucket" "this" {
  project                     = var.project_id
  name                        = var.bucket_name
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = var.force_destroy

  versioning {
    enabled = false
  }

  encryption {
    default_kms_key_name = google_kms_crypto_key.bucket.id
  }

  lifecycle_rule {
    action {
      type = "Delete"
    }
    condition {
      age = var.object_ttl_days
    }
  }

  depends_on = [google_kms_crypto_key_iam_member.gcs_sa_encrypter]
}
