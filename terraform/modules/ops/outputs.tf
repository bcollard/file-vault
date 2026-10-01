output "sweeper_job_name" {
  value = google_cloud_run_v2_job.sweeper.name
}

output "scheduler_job_name" {
  value = google_cloud_scheduler_job.sweep.name
}

output "notification_channel_id" {
  value = google_monitoring_notification_channel.email.id
}
