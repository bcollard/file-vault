output "neg_id" {
  description = "Serverless NEG for the file-vault service. Attach it as the only backend for your hostname — see docs/lb-setup.md."
  value       = module.app.neg_id
}

output "service_url" {
  description = "*.run.app URL. Ingress is load-balancer-only, so this is not reachable directly."
  value       = module.app.service_url
}

output "bucket_name" {
  value = module.storage.bucket_name
}

output "service_account" {
  value = module.app.service_account
}

output "public_url" {
  value = "https://${var.domain}"
}
