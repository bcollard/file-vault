locals {
  env = "prod"

  required_apis = [
    "run.googleapis.com",
    "firestore.googleapis.com",
    "storage.googleapis.com",
    "secretmanager.googleapis.com",
    "cloudkms.googleapis.com",
    "iam.googleapis.com",
    "iamcredentials.googleapis.com",
    "compute.googleapis.com",
    "monitoring.googleapis.com",
    "logging.googleapis.com",
    "artifactregistry.googleapis.com",
  ]
}

resource "google_project_service" "apis" {
  for_each = toset(local.required_apis)

  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

module "secrets" {
  source = "../../modules/secrets"

  project_id = var.project_id
  env        = local.env

  depends_on = [google_project_service.apis]
}

# Objects live exactly as long as shares: the lifecycle rule and Firestore TTL
# do all expiry, so no sweeper job is needed.
module "storage" {
  source = "../../modules/storage"

  project_id      = var.project_id
  region          = var.region
  env             = local.env
  bucket_name     = var.bucket_name
  object_ttl_days = var.share_ttl_days
  force_destroy   = var.force_destroy_bucket
  cors_origins    = ["https://${var.domain}"]

  depends_on = [google_project_service.apis]
}

module "firestore" {
  source = "../../modules/firestore"

  project_id = var.project_id
  region     = var.region
  env        = local.env

  depends_on = [google_project_service.apis]
}

module "app" {
  source = "../../modules/app"

  project_id         = var.project_id
  region             = var.region
  env                = local.env
  image              = var.image
  bucket_name        = module.storage.bucket_name
  firestore_database = module.firestore.database_name
  pepper_secret_id   = module.secrets.pepper_secret_id
  public_domain      = var.domain
  min_instances      = var.min_instances
  max_instances      = var.max_instances
  share_ttl_days     = var.share_ttl_days
  max_upload_mb      = var.max_upload_mb
}

module "ops" {
  source = "../../modules/ops"

  project_id   = var.project_id
  region       = var.region
  env          = local.env
  alert_email  = var.alert_email
  service_name = module.app.service_name
}
