resource "google_monitoring_notification_channel" "email" {
  project      = var.project_id
  display_name = "file-vault ops ${var.env}"
  type         = "email"

  labels = {
    email_address = var.alert_email
  }
}

# Count rejected reuse / unknown-token requests. The API emits structured logs
# with jsonPayload.event = "token_reuse_rejected" — see handleBurn in api/internal/server.
resource "google_logging_metric" "rejected_reuse" {
  project = var.project_id
  name    = "file-vault/rejected_reuse_${var.env}"
  filter  = "resource.type=\"cloud_run_revision\" AND resource.labels.service_name=\"${var.service_name}\" AND jsonPayload.event=\"token_reuse_rejected\""

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"
  }
}

# A new log-based metric takes a few minutes before alert policies can
# reference it; without this wait the first apply fails with a 404.
resource "time_sleep" "metric_propagation" {
  create_duration = "180s"

  triggers = {
    metric = google_logging_metric.rejected_reuse.id
  }
}

resource "google_monitoring_alert_policy" "reuse_spike" {
  depends_on = [time_sleep.metric_propagation]

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
