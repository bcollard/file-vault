locals {
  env = "dev"

  required_apis = [
    "run.googleapis.com",
    "firestore.googleapis.com",
    "storage.googleapis.com",
    "secretmanager.googleapis.com",
    "cloudkms.googleapis.com",
    "cloudscheduler.googleapis.com",
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

module "network" {
  source = "../../modules/network"

  project_id = var.project_id
  region     = var.region
  env        = local.env

  depends_on = [google_project_service.apis]
}

module "secrets" {
  source = "../../modules/secrets"

  project_id = var.project_id
  env        = local.env

  depends_on = [google_project_service.apis]
}

module "storage" {
  source = "../../modules/storage"

  project_id      = var.project_id
  region          = var.region
  env             = local.env
  bucket_name     = var.bucket_name
  object_ttl_days = var.object_ttl_days
  force_destroy   = var.force_destroy_bucket

  depends_on = [google_project_service.apis]
}

module "firestore" {
  source = "../../modules/firestore"

  project_id = var.project_id
  region     = var.region
  env        = local.env

  depends_on = [google_project_service.apis]
}

module "api" {
  source = "../../modules/api"

  project_id       = var.project_id
  region           = var.region
  env              = local.env
  image            = var.api_image
  bucket_name      = module.storage.bucket_name
  pepper_secret_id = module.secrets.pepper_secret_id
  vpc_subnet_id    = module.network.subnet_id
  share_ttl_days   = var.share_ttl_days

  depends_on = [module.firestore]
}

module "web" {
  source = "../../modules/web"

  project_id    = var.project_id
  region        = var.region
  env           = local.env
  image         = var.web_image
  api_url       = module.api.service_url
  api_service   = module.api.service_name
  vpc_subnet_id = module.network.subnet_id
  public_domain = var.domain
}

module "ops" {
  source = "../../modules/ops"

  project_id    = var.project_id
  region        = var.region
  env           = local.env
  bucket_name   = module.storage.bucket_name
  alert_email   = var.alert_email
  api_service   = module.api.service_name
  sweeper_image = var.sweeper_image
}
