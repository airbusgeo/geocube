terraform {
  required_version = ">= 0.12.18"
  required_providers {
    kubernetes = {
      source = "hashicorp/kubernetes"
      version = ">= 1.11"
    }
    google = {
      version = ">= 3.8.0"
    }
    google-beta = {
      version = ">= 3.8.0"
    }
    random = {
      version = "~> 2.2"
    }
    tls = {
      version = "~> 2.1"
    }
  }
}

provider "google" {
  region = var.region
  project = var.project-id
  zone = var.zone
}

provider "google-beta" {
  region = var.region
  project = var.project-id
  zone = var.zone
}

data "google_client_config" "provider" {}

data "google_container_cluster" "geocube_cluster" {
  project = var.project-id
  name = var.cluster-name
  location = var.cluster-location
}

provider "kubernetes" {
  host  = "https://${data.google_container_cluster.geocube_cluster.endpoint}"
  token                  = data.google_client_config.provider.access_token
  cluster_ca_certificate = base64decode(
    data.google_container_cluster.geocube_cluster.master_auth[0].cluster_ca_certificate
  )
}

provider "random" {
}
provider "tls" {
}

resource "google_compute_global_address" "geocube-gw-address" {
  name = "geocube-gw-address"
}

resource "google_project_service" "project-api" {
  for_each = toset(var.api-services)
  service = each.value
  disable_dependent_services = true
}

resource "google_compute_address" "geocube-address" {
  name = "geocube-address"
}
