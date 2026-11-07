output "neg_id" {
  value = google_compute_region_network_endpoint_group.app.id
}

output "service_url" {
  value = google_cloud_run_v2_service.app.uri
}

output "service_name" {
  value = google_cloud_run_v2_service.app.name
}

output "service_account" {
  value = google_service_account.app.email
}
