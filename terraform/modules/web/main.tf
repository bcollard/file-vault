resource "google_service_account" "web" {
  project      = var.project_id
  account_id   = "file-vault-web-${var.env}"
  display_name = "file-vault web runtime"
}

# Web service mints ID tokens for itself and calls the API over the internal mesh.
resource "google_cloud_run_v2_service_iam_member" "web_invokes_api" {
  project  = var.project_id
  location = var.region
  name     = var.api_service
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.web.email}"
}

resource "google_cloud_run_v2_service" "web" {
  project  = var.project_id
  name     = "file-vault-web-${var.env}"
  location = var.region

  ingress = "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER"

  template {
    service_account = google_service_account.web.email

    # ALL_TRAFFIC so calls to <api>.run.app egress through the VPC and satisfy the API's internal-ingress check.
    vpc_access {
      egress = "ALL_TRAFFIC"
      network_interfaces {
        subnetwork = var.vpc_subnet_id
      }
    }

    scaling {
      min_instance_count = var.min_instances
      max_instance_count = var.max_instances
    }

    containers {
      image = var.image

      env {
        name  = "API_URL"
        value = var.api_url
      }
      env {
        name  = "API_AUDIENCE"
        value = var.api_url
      }
      env {
        name  = "PUBLIC_DOMAIN"
        value = var.public_domain
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
      }
    }
  }
}

resource "google_cloud_run_v2_service_iam_member" "web_public" {
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.web.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

resource "google_compute_region_network_endpoint_group" "web" {
  project               = var.project_id
  name                  = "file-vault-web-${var.env}"
  region                = var.region
  network_endpoint_type = "SERVERLESS"

  cloud_run {
    service = google_cloud_run_v2_service.web.name
  }
}
