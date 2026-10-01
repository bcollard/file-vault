output "neg_id" {
  value = google_compute_region_network_endpoint_group.web.id
}

output "service_url" {
  value = google_cloud_run_v2_service.web.uri
}

output "service_name" {
  value = google_cloud_run_v2_service.web.name
}

output "service_account" {
  value = google_service_account.web.email
}
