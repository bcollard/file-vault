resource "google_service_account" "api" {
  project      = var.project_id
  account_id   = "file-vault-api-${var.env}"
  display_name = "file-vault API runtime"
}

resource "google_storage_bucket_iam_member" "api_object_admin" {
  bucket = var.bucket_name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.api.email}"
}

resource "google_project_iam_member" "api_datastore" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.api.email}"
}

# Self-impersonation for IAM SignBlob — V4 signed URLs without a JSON key on disk.
resource "google_service_account_iam_member" "api_self_token_creator" {
  service_account_id = google_service_account.api.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:${google_service_account.api.email}"
}

resource "google_secret_manager_secret_iam_member" "api_pepper_reader" {
  project   = var.project_id
  secret_id = var.pepper_secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.api.email}"
}

resource "google_cloud_run_v2_service" "api" {
  project  = var.project_id
  name     = "file-vault-api-${var.env}"
  location = var.region

  # Reachable from both the external LB (for /d/* recipients) and the VPC (for the frontend calling internal endpoints).
  ingress = "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER"

  template {
    service_account = google_service_account.api.email

    vpc_access {
      egress = "PRIVATE_RANGES_ONLY"
      network_interfaces {
        subnetwork = var.vpc_subnet_id
      }
    }

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
        value = "(default)"
      }
      env {
        name  = "SIGNER_SERVICE_ACCOUNT"
        value = google_service_account.api.email
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
          memory = "512Mi"
        }
      }
    }
  }
}

# Capability tokens are the auth model — LB NEGs don't pass IAM, so allow public invoke
# at the Cloud Run layer and let the app enforce token + ID-token checks.
resource "google_cloud_run_v2_service_iam_member" "api_public" {
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.api.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

resource "google_compute_region_network_endpoint_group" "api" {
  project               = var.project_id
  name                  = "file-vault-api-${var.env}"
  region                = var.region
  network_endpoint_type = "SERVERLESS"

  cloud_run {
    service = google_cloud_run_v2_service.api.name
  }
}
