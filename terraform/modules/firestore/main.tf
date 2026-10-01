resource "google_firestore_database" "default" {
  project                 = var.project_id
  name                    = "(default)"
  location_id             = var.region
  type                    = "FIRESTORE_NATIVE"
  delete_protection_state = "DELETE_PROTECTION_DISABLED"
}

resource "google_firestore_field" "shares_ttl" {
  project    = var.project_id
  database   = google_firestore_database.default.name
  collection = "shares"
  field      = "expires_at"

  ttl_config {}

  # Don't manage default single-field indexes from Terraform — leave Firestore's defaults alone.
  index_config {}
}

resource "google_firestore_field" "download_tokens_ttl" {
  project    = var.project_id
  database   = google_firestore_database.default.name
  collection = "download_tokens"
  field      = "expires_at"

  ttl_config {}

  index_config {}
}

resource "google_firestore_field" "admin_tokens_ttl" {
  project    = var.project_id
  database   = google_firestore_database.default.name
  collection = "admin_tokens"
  field      = "expires_at"

  ttl_config {}

  index_config {}
}

# Admin token list: download tokens per share, newest first.
resource "google_firestore_index" "download_tokens_by_share" {
  project    = var.project_id
  database   = google_firestore_database.default.name
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

# Admin console timeline: events per share, newest first.
resource "google_firestore_index" "audit_by_share" {
  project    = var.project_id
  database   = google_firestore_database.default.name
  collection = "audit_events"

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
