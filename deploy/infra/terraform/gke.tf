resource "google_service_account" "gke-node" {
  account_id = "gke-node"
}

resource "google_project_iam_member" "service_account-log_writer" {
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.gke-node.email}"
}

resource "google_project_iam_member" "service_account-metric_writer" {
  role    = "roles/monitoring.metricWriter"
  member  = "serviceAccount:${google_service_account.gke-node.email}"
}

resource "google_project_iam_member" "service_account-monitoring_viewer" {
  role    = "roles/monitoring.viewer"
  member  = "serviceAccount:${google_service_account.gke-node.email}"
}

resource "google_storage_bucket_iam_member" "gke-gcr-reader" {
  bucket = "eu.artifacts.${var.project-id}.appspot.com"
  role = "roles/storage.objectViewer"
  member = "serviceAccount:${google_service_account.gke-node.email}"
}

resource "google_container_cluster" "geocube" {
  name     = var.cluster
  location = var.zone
  provider = google-beta
  remove_default_node_pool = true
  initial_node_count = 1
  network = google_compute_network.geocube-net.name
  subnetwork = google_compute_subnetwork.geocube-subnet.name
  node_config {
    service_account = google_service_account.gke-node.email
    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform"
    ]
  }
  addons_config {
    http_load_balancing {
      disabled = false
    } 

    horizontal_pod_autoscaling {
      disabled = false
    }

    #kubernetes_dashboard {
    #  disabled = true
    #}
  }
  release_channel {
      channel = "REGULAR"
  }

  ip_allocation_policy {
    cluster_secondary_range_name = "pods"
    services_secondary_range_name = "services"
  }

  master_auth {
    username = ""
    password = ""

    client_certificate_config {
      issue_client_certificate = false
    }
  }

  #master_authorized_networks_config {
  #  cidr_blocks {
  #    cidr_block = "213.190.75.99/32"
  #    display_name = "ads geo"
  #  }
  #}

  cluster_autoscaling {
    enabled = false
  }
  lifecycle {
    ignore_changes = [ node_config ]
  }

}

resource "google_container_node_pool" "main" {
  provider = google-beta
  name       = "main"
  location   = var.zone
  cluster    = google_container_cluster.geocube.name
  initial_node_count = 1

  autoscaling {
    min_node_count = "1"
    max_node_count = "5"
  }

  management {
    auto_repair  = "true"
    auto_upgrade = "true"
  }

  node_config {
    image_type   = "COS"
    preemptible  = true
    machine_type = "n1-standard-2"
    disk_size_gb = "50"
    disk_type = "pd-ssd"
    service_account = google_service_account.gke-node.email

    metadata = {
      disable-legacy-endpoints = "true"
    }
    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform"
    ]
  }
  lifecycle {
    ignore_changes = [initial_node_count, node_count]
  }
}
resource "google_container_node_pool" "preemptible" {
  provider = google-beta
  name       = "preemptible"
  location   = var.zone
  cluster    = google_container_cluster.geocube.name
  initial_node_count = 0

  autoscaling {
    min_node_count = "0"
    max_node_count = "1024"
  }

  management {
    auto_repair  = "true"
    auto_upgrade = "true"
  }


  node_config {
    image_type   = "COS"
    preemptible  = true
    machine_type = "n1-standard-16"
    disk_size_gb = "50"
    disk_type = "pd-ssd"
    service_account = google_service_account.gke-node.email
    local_ssd_count = 1
    taint {
      key = "preemptible"
      value = "true"
      effect = "NO_SCHEDULE"
    }

    metadata = {
      disable-legacy-endpoints = "true"
    }
    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform"
    ]
  }
  lifecycle {
    ignore_changes = [initial_node_count, node_count]
  }
}

data "google_client_config" "provider" {
}

provider "kubernetes" {
  host = "https://${google_container_cluster.geocube.endpoint}"
  token = data.google_client_config.provider.access_token
  cluster_ca_certificate = base64decode(
    google_container_cluster.geocube.master_auth[0].cluster_ca_certificate,
  )
}

resource "kubernetes_config_map" "geocube-config" {
  metadata {
    name = "geocube-config"
  }
  data = {
    "db_name": var.db-name
    "cloudsql_ip": google_sql_database_instance.instance.private_ip_address
    "db_root_user" : google_sql_user.geocube-root.name
    "db_apiserver_user" : var.db-apiserver-user
    "bucket": google_storage_bucket.geocube.name
    "temp_bucket": google_storage_bucket.geocube-temp.name
    "project": var.project-id
    "region": var.region
    "zone": var.zone
    "consolidations_subscription": google_pubsub_subscription.consolidations-worker.name
    "events_topic": google_pubsub_topic.events.name
  }
}
