resource "google_compute_network" "geocube-net" {
  name          =  "geocube-dev"
  auto_create_subnetworks = "false"
  routing_mode            = "GLOBAL"
}
resource "google_compute_subnetwork" "geocube-subnet" {
  name          = "geocube-subnet"
  region        = var.region
  network       = google_compute_network.geocube-net.self_link
  ip_cidr_range = var.subnet-range
  private_ip_google_access = true
  secondary_ip_range = [
      {
          range_name = "pods"
          ip_cidr_range = var.pod-range
      },
      {
          range_name = "services"
          ip_cidr_range = var.svc-range
      },

  ]
}

resource "google_compute_firewall" "allow-internal" {
  name    = "geocube-allow-internal"
  network = google_compute_network.geocube-net.name
  allow {
    protocol = "icmp"
  }
  allow {
    protocol = "tcp"
    ports    = ["0-65535"]
  }
  allow {
    protocol = "udp"
    ports    = ["0-65535"]
  }
  source_ranges = [
    var.subnet-range,
    var.pod-range,
    var.svc-range
  ]
}