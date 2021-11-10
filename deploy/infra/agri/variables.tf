variable "project-id" {
}
variable "cluster-name" {
}
variable "cluster-location" {
  default="europe-west1-d"
}
variable "short-sha" {
}
variable "registry" {
}
variable "namespace" {
}
variable "network" {
  description = "projects/{project}/global/networks/{networkName}"
}
variable "region" {
  default="europe-west1"
}
variable "zone" {
  default="europe-west1-d"
}
variable "db-name" {
  default = "geocube"
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
variable "subnet-range" {
  default="10.130.0.0/20"
}
variable "pod-range" {
  default="10.96.0.0/14"
}
variable "svc-range" {
  default="10.94.0.0/18"
}
variable "db-root-user" {
  default="geocube-root"
}
variable "db-apiserver-user" {
  default="apiserver"
}
variable "endpoint-fqdn" {
}
variable "db-credentials-secret-name" {
  default = "db-credentials"
}
variable "bearer-auth-secret-name" {
  default = "geocube-bearer-auth"
}
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