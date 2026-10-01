output "api_neg_id" {
  value = module.api.neg_id
}

output "web_neg_id" {
  value = module.web.neg_id
}

output "api_service_url" {
  value = module.api.service_url
}

output "web_service_url" {
  value = module.web.service_url
}

output "bucket_name" {
  value = module.storage.bucket_name
}

output "api_service_account" {
  value = module.api.service_account
}

output "web_service_account" {
  value = module.web.service_account
}
