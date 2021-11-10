variable "project-id" {
}
variable "region" {
  default="europe-west1"
}
variable "zone" {
  default="europe-west1-d"
}
variable "subnet-range" {
  default="10.130.0.0/20"
}
variable "pod-range" {
  default="10.96.0.0/14"
}
variable "svc-range" {
  default="10.94.0.0/18"
}
variable "cluster" {
  default="geocube"
}
variable "consolidation-topic" {
  default="consolidations"
}
variable "consolidation-worker-subscription" {
  default="consolidations-worker"
}
variable "events-topic" {
  default="events"
}
variable "events-subscription" {
  default="events"
}
variable "db-root-user" {
  default="geocube-root"
}
variable "db-apiserver-user" {
  default="apiserver"
}
variable "db-name" {
  default = "geocube"
}
variable "db-credentials-secret-name" {
  default = "db-credentials"
}
variable "bearer-auth-secret-name" {
  default = "geocube-bearer-auth"
}
variable "short-sha" {
}
variable "registry" {
}
variable "endpoint-fqdn" {
  default = "dev.geocube.airbusds-geo.com"
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
provider "random" {
}
provider "tls" {
}

terraform {
  required_version = ">= 0.12.18"
  backend "gcs" {
  }
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
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

#resource "google_project" "geocube" {
#  name = var.project-id
#  project_id = var.project-id
#}

variable "api-services" {
  default = [
	 "pubsub.googleapis.com",
	 "compute.googleapis.com",
	 "container.googleapis.com",
	 "cloudresourcemanager.googleapis.com",
	 "monitoring.googleapis.com",
	 "logging.googleapis.com",
	 "run.googleapis.com",
	 "sql-component.googleapis.com",
	 "cloudfunctions.googleapis.com",
	 "containerregistry.googleapis.com",
	 "servicenetworking.googleapis.com",
	 "sqladmin.googleapis.com",
   "secretmanager.googleapis.com"
  ]
}
resource "google_project_service" "project-api" {
  for_each = toset(var.api-services)
  service = each.value
  disable_dependent_services = true
}

resource "random_id" "bucket_suffix" {
  byte_length = 4
}
resource "google_storage_bucket" "geocube" {
  name     = "geocube-${random_id.bucket_suffix.hex}"
  location = var.region
  storage_class = "REGIONAL"
}
resource "google_storage_bucket" "geocube-temp" {
  name     = "geocube-temp-${random_id.bucket_suffix.hex}"
  location = var.region
  storage_class = "REGIONAL"
  lifecycle_rule {
    action {
      type = "Delete"
    }
    condition {
      age = 30
    }
  }
}

resource "google_compute_address" "geocube-address" {
  name = "geocube-address"
}

resource "google_compute_global_address" "geocube-gw-address" {
  name = "geocube-gw-address"
}
