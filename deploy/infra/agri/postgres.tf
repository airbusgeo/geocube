resource "google_compute_global_address" "postgres_private_ip_address" {
  provider = google-beta

  name          = "private-ip-address"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 16
  network       = var.network
}

resource "google_service_networking_connection" "postgresql_private_vpc_connection" {
  provider = google-beta

  network                 = var.network
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.postgres_private_ip_address.name]
}

variable "database-flags" {
  type    = map
  default = {
    max_connections = "1024"
  }
}

resource "random_id" "db_name_suffix" {
  byte_length = 4
}

resource "google_sql_database_instance" "instance" {
  provider = google-beta

  name   = "geocube-instance-${random_id.db_name_suffix.hex}"
  region = var.region

  depends_on = [google_service_networking_connection.postgresql_private_vpc_connection]
   
  database_version = "POSTGRES_11"

  settings {
    tier = "db-custom-1-3840"
    ip_configuration {
      ipv4_enabled    = false
      private_network = var.network
      require_ssl = false

    }
    dynamic "database_flags" {
      iterator = flag
      for_each = var.database-flags

      content {
        name = flag.key
        value = flag.value
      }
    }
  }
}

resource "google_sql_database" "geocube" {
  name     = var.db-name
  instance = google_sql_database_instance.instance.name
}

resource "random_password" "geocube-root-password" {
  length = 16
  special = true
  override_special = "_%@"
}

resource "random_password" "apiserver-password" {
  length = 16
  special = true
  override_special = "_%@"
}

resource "google_sql_user" "geocube-root" {
  name     = var.db-root-user
  instance = google_sql_database_instance.instance.name
  password = random_password.geocube-root-password.result
}

resource "google_sql_user" "geocube-user" {
  name = var.db-apiserver-user
  instance = google_sql_database_instance.instance.name
  password = random_password.apiserver-password.result
}

resource "kubernetes_secret" "db" {
  type = "Opaque"
  metadata {
    name = "db"
    namespace = kubernetes_namespace.geocube.metadata[0].name
  }
  data = {
    "db_root_password" = google_sql_user.geocube-root.password
    "db_apiserver_password" = random_password.apiserver-password.result
    "db_root_connection_string" = "postgres://${google_sql_user.geocube-root.name}:${google_sql_user.geocube-root.password}@${google_sql_database_instance.instance.private_ip_address}/${google_sql_database.geocube.name}"
  }
}