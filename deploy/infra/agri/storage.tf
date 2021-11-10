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