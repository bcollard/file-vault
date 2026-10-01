resource "google_compute_network" "vpc" {
  project                 = var.project_id
  name                    = "file-vault-${var.env}"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "subnet" {
  project       = var.project_id
  name          = "file-vault-${var.env}"
  region        = var.region
  network       = google_compute_network.vpc.id
  ip_cidr_range = var.subnet_cidr

  private_ip_google_access = true
}
