resource "google_service_account" "geocube-server" {
  account_id   = "geocube-server"
  display_name = "api server Account"
}

resource "google_service_account" "consolidater" {
  account_id   = "consolidater"
  display_name = "Consolidater Account"
}

resource "google_project_iam_member" "geocube-server-secret-accessor" {
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${google_service_account.geocube-server.email}"
}

resource "google_storage_bucket_iam_member" "apiserver-bucket-admin" {
  bucket = google_storage_bucket.geocube.name
  role = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.geocube-server.email}"
}

resource "google_project_iam_member" "consolidater-pubsub-publisher" {
  role    = "roles/pubsub.publisher"
  member  = "serviceAccount:${google_service_account.consolidater.email}"
}

resource "google_project_iam_member" "consolidater-pubsub-subscriber" {
  role    = "roles/pubsub.subscriber"
  member  = "serviceAccount:${google_service_account.consolidater.email}"
}

resource "google_storage_bucket_iam_member" "consolidater-bucket-admin" {
  bucket = google_storage_bucket.geocube.name
  role = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.consolidater.email}"
}

resource "google_project_iam_member" "geocube-server-pubsub-publisher" {
  role    = "roles/pubsub.publisher"
  member  = "serviceAccount:${google_service_account.geocube-server.email}"
}

resource "google_project_iam_member" "geocube-server-pubsub-subscriber" {
  role    = "roles/pubsub.subscriber"
  member  = "serviceAccount:${google_service_account.geocube-server.email}"
}
