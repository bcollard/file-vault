output "api_neg_id" {
  description = "Serverless NEG for the API. Wire into your external LB backend service with a path matcher for /d/* (and /api/* if you expose internal endpoints publicly)."
  value       = module.api.neg_id
}

output "web_neg_id" {
  description = "Serverless NEG for the web UI. Wire as the default backend for everything that isn't /d/*."
  value       = module.web.neg_id
}

output "api_service_url" {
  description = "Internal *.run.app URL of the API service. Web service uses this as both target URL and ID-token audience."
  value       = module.api.service_url
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
