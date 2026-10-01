output "pepper_secret_id" {
  value = google_secret_manager_secret.pepper.secret_id
}

output "pepper_secret_name" {
  value = google_secret_manager_secret.pepper.name
}
