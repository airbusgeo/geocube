resource "google_sql_database" "sar" {
  name     = "sar"
  instance = google_sql_database_instance.instance.name
}

resource "random_password" "sar-password" {
  length = 16
  special = true
  override_special = "_%@"
}

resource "google_sql_user" "sar" {
  name     = "sar"
  instance = google_sql_database_instance.instance.name
  password = random_password.sar-password.result
}

resource "kubernetes_secret" "sar-db" {
  type = "Opaque"
  metadata {
    name = "sar-db"
    namespace = "sar"
  }
  data = {
    "password" = google_sql_user.sar.password
    "user" = "sar"
    "connection_string" = "postgres://${google_sql_user.sar.name}:${google_sql_user.sar.password}@${google_sql_database_instance.instance.private_ip_address}/${google_sql_database.sar.name}"
  }
}