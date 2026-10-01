output "neg_id" {
  value = google_compute_region_network_endpoint_group.api.id
}

output "service_url" {
  value = google_cloud_run_v2_service.api.uri
}

output "service_name" {
  value = google_cloud_run_v2_service.api.name
}

output "service_account" {
  value = google_service_account.api.email
}
