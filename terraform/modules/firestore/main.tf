# A named database, so file-vault never shares (or collides with) a project's
# (default) database used by other apps.
resource "google_firestore_database" "this" {
  project                 = var.project_id
  name                    = "file-vault-${var.env}"
  location_id             = var.region
  type                    = "FIRESTORE_NATIVE"
  delete_protection_state = var.delete_protection ? "DELETE_PROTECTION_ENABLED" : "DELETE_PROTECTION_DISABLED"
  deletion_policy         = var.delete_protection ? "ABANDON" : "DELETE"
}

resource "google_firestore_field" "shares_ttl" {
  project    = var.project_id
  database   = google_firestore_database.this.name
  collection = "shares"
  field      = "expires_at"

  ttl_config {}

  # Don't manage default single-field indexes from Terraform — leave Firestore's defaults alone.
  index_config {}
}

resource "google_firestore_field" "download_tokens_ttl" {
  project    = var.project_id
  database   = google_firestore_database.this.name
  collection = "download_tokens"
  field      = "expires_at"

  ttl_config {}

  index_config {}
}

resource "google_firestore_field" "admin_tokens_ttl" {
  project    = var.project_id
  database   = google_firestore_database.this.name
  collection = "admin_tokens"
  field      = "expires_at"

  ttl_config {}

  index_config {}
}

# Admin token list: download tokens per share, newest first.
resource "google_firestore_index" "download_tokens_by_share" {
  project    = var.project_id
  database   = google_firestore_database.this.name
  collection = "download_tokens"

  fields {
    field_path = "share_id"
    order      = "ASCENDING"
  }

  fields {
    field_path = "created_at"
    order      = "DESCENDING"
  }

  fields {
    field_path = "__name__"
    order      = "DESCENDING"
  }
}

# Audit events carry client IPs; drop them after the retention window
# (AUDIT_RETENTION_DAYS sets expires_at on each event).
resource "google_firestore_field" "audit_events_ttl" {
  project    = var.project_id
  database   = google_firestore_database.this.name
  collection = "audit_events"
  field      = "expires_at"

  ttl_config {}

  index_config {}
}
