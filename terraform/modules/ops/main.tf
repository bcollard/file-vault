resource "google_service_account" "sweeper" {
  project      = var.project_id
  account_id   = "file-vault-sweeper-${var.env}"
  display_name = "file-vault sweeper job"
}

resource "google_storage_bucket_iam_member" "sweeper_object_admin" {
  bucket = var.bucket_name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.sweeper.email}"
}

resource "google_project_iam_member" "sweeper_datastore" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.sweeper.email}"
}

resource "google_cloud_run_v2_job" "sweeper" {
  project  = var.project_id
  name     = "file-vault-sweeper-${var.env}"
  location = var.region

  template {
    template {
      service_account = google_service_account.sweeper.email
      timeout         = "600s"

      containers {
        image = var.sweeper_image

        env {
          name  = "BUCKET_NAME"
          value = var.bucket_name
        }
        env {
          name  = "FIRESTORE_DATABASE"
          value = "(default)"
        }
      }
    }
  }
}

resource "google_service_account" "scheduler" {
  project      = var.project_id
  account_id   = "file-vault-scheduler-${var.env}"
  display_name = "file-vault scheduler invoker"
}

resource "google_cloud_run_v2_job_iam_member" "scheduler_runs_sweeper" {
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.sweeper.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.scheduler.email}"
}

resource "google_cloud_scheduler_job" "sweep" {
  project   = var.project_id
  region    = var.region
  name      = "file-vault-sweep-${var.env}"
  schedule  = var.sweep_cron
  time_zone = "UTC"

  http_target {
    http_method = "POST"
    uri         = "https://${var.region}-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/${var.project_id}/jobs/${google_cloud_run_v2_job.sweeper.name}:run"

    oauth_token {
      service_account_email = google_service_account.scheduler.email
    }
  }
}

resource "google_monitoring_notification_channel" "email" {
  project      = var.project_id
  display_name = "file-vault ops ${var.env}"
  type         = "email"

  labels = {
    email_address = var.alert_email
  }
}

# Count rejected reuse / unknown-token requests. The API emits structured logs
# with jsonPayload.event = "token_reuse_rejected" — see Go API handler design.
resource "google_logging_metric" "rejected_reuse" {
  project = var.project_id
  name    = "file-vault/rejected_reuse_${var.env}"
  filter  = "resource.type=\"cloud_run_revision\" AND resource.labels.service_name=\"${var.api_service}\" AND jsonPayload.event=\"token_reuse_rejected\""

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"
  }
}

resource "google_monitoring_alert_policy" "reuse_spike" {
  project      = var.project_id
  display_name = "file-vault: rejected reuse spike (${var.env})"
  combiner     = "OR"

  conditions {
    display_name = "rejected_reuse > ${var.reuse_threshold} in 5min"
    condition_threshold {
      filter          = "metric.type=\"logging.googleapis.com/user/${google_logging_metric.rejected_reuse.name}\" resource.type=\"cloud_run_revision\""
      comparison      = "COMPARISON_GT"
      threshold_value = var.reuse_threshold
      duration        = "300s"

      aggregations {
        alignment_period   = "60s"
        per_series_aligner = "ALIGN_SUM"
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]
}
