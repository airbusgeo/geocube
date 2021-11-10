resource "google_pubsub_topic" "events" {
  name = var.events-topic
  message_storage_policy {
    allowed_persistence_regions = [
      var.region,
    ]
  }
}

resource "google_pubsub_topic" "events-failed" {
  name = "${var.events-topic}-failed"
  message_storage_policy {
    allowed_persistence_regions = [
      var.region,
    ]
  }
}

resource "google_pubsub_topic" "consolidations" {
  name = var.consolidation-topic
  message_storage_policy {
    allowed_persistence_regions = [
      var.region,
    ]
  }
}

resource "google_pubsub_topic" "consolidations-failed" {
  name = "${var.consolidation-topic}-failed"
  message_storage_policy {
    allowed_persistence_regions = [
      var.region,
    ]
  }
}

resource "google_pubsub_subscription" "events" {
  name  = var.events-subscription
  topic = google_pubsub_topic.events.name

  ack_deadline_seconds = 600

  expiration_policy {
    ttl = ""
  }

  push_config {
    push_endpoint = "https://${var.endpoint-fqdn}/push"
    oidc_token {
      service_account_email = google_service_account.geocube-server.email
    }
  }
  dead_letter_policy {
    dead_letter_topic = google_pubsub_topic.events-failed.id
    max_delivery_attempts = 100
    //FIXME: need to grant publish role to failed topic, and subscriber role to this topic to the gcp managed pubsub serviceaccount
  }
}

resource "google_pubsub_subscription" "consolidations-worker" {
  name  = var.consolidation-worker-subscription
  topic = google_pubsub_topic.consolidations.name

  ack_deadline_seconds = 600

  expiration_policy {
    ttl = ""
  }
  dead_letter_policy {
    dead_letter_topic = google_pubsub_topic.consolidations-failed.id
    max_delivery_attempts = 10
    //FIXME: need to grant publish role to failed topic, and subscriber role to this topic to the gcp managed pubsub serviceaccount
  }
}