resource "google_service_account" "app" {
  project      = var.project_id
  account_id   = "file-vault-${var.env}"
  display_name = "file-vault runtime"
}

# Opens resumable upload sessions, stats and deletes objects. The browser
# writes ciphertext to the session URL directly; the service never proxies bytes.
resource "google_storage_bucket_iam_member" "app_object_admin" {
  bucket = var.bucket_name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.app.email}"
}

resource "google_project_iam_member" "app_datastore" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.app.email}"
}

# Self-impersonation for IAM SignBlob — V4 signed URLs without a JSON key on disk.
resource "google_service_account_iam_member" "app_self_token_creator" {
  service_account_id = google_service_account.app.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:${google_service_account.app.email}"
}

resource "google_secret_manager_secret_iam_member" "app_pepper_reader" {
  project   = var.project_id
  secret_id = var.pepper_secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.app.email}"
}

resource "google_cloud_run_v2_service" "app" {
  project  = var.project_id
  name     = "file-vault-${var.env}"
  location = var.region

  # Only the external load balancer can reach the service. TRUSTED_PROXY_HOPS
  # below assumes exactly that path when reading X-Forwarded-For.
  ingress = "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER"

  template {
    service_account = google_service_account.app.email

    scaling {
      min_instance_count = var.min_instances
      max_instance_count = var.max_instances
    }

    containers {
      image = var.image

      env {
        name  = "BUCKET_NAME"
        value = var.bucket_name
      }
      env {
        name  = "FIRESTORE_DATABASE"
        value = var.firestore_database
      }
      env {
        name  = "SIGNER_SERVICE_ACCOUNT"
        value = google_service_account.app.email
      }
      env {
        name  = "PUBLIC_ORIGIN"
        value = "https://${var.public_domain}"
      }
      env {
        name  = "DOWNLOAD_URL_TTL_SECONDS"
        value = tostring(var.download_url_ttl_seconds)
      }
      env {
        name  = "SHARE_TTL_DAYS"
        value = tostring(var.share_ttl_days)
      }
      env {
        name  = "AUDIT_RETENTION_DAYS"
        value = tostring(var.audit_retention_days)
      }
      env {
        name  = "MAX_UPLOAD_MB"
        value = tostring(var.max_upload_mb)
      }
      env {
        name  = "TRUSTED_PROXY_HOPS"
        value = tostring(var.trusted_proxy_hops)
      }
      env {
        name = "HMAC_PEPPER"
        value_source {
          secret_key_ref {
            secret  = var.pepper_secret_id
            version = "latest"
          }
        }
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "256Mi"
        }
        cpu_idle = true
      }

      startup_probe {
        http_get {
          path = "/healthz"
        }
      }
    }
  }
}

# Capability tokens are the auth model and LB serverless NEGs don't carry IAM
# identity, so allow public invoke at the Cloud Run layer. Ingress above still
# restricts traffic to the load balancer.
resource "google_cloud_run_v2_service_iam_member" "app_public" {
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.app.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

resource "google_compute_region_network_endpoint_group" "app" {
  project               = var.project_id
  name                  = "file-vault-${var.env}"
  region                = var.region
  network_endpoint_type = "SERVERLESS"

  cloud_run {
    service = google_cloud_run_v2_service.app.name
  }
}
