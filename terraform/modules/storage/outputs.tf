output "bucket_name" {
  value = google_storage_bucket.this.name
}

output "bucket_self_link" {
  value = google_storage_bucket.this.self_link
}

output "kms_key_id" {
  value = google_kms_crypto_key.bucket.id
}
