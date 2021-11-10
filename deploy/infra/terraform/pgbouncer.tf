/*
resource "kubernetes_secret" "pgbouncer" {
  type = "Opaque"
  metadata {
    name = "pgbouncer"
  }
  data = {
    "pgbouncer.crt" = tls_locally_signed_cert.pgbouncer.cert_pem
    "pgbouncer.key" = tls_private_key.pgbouncer.private_key_pem
    "root.crt" = tls_self_signed_cert.root.cert_pem
    "pgbouncer.ini" =<<EOF
[databases]
  ${var.db-name} = host=${google_sql_database_instance.instance.private_ip_address} dbname=${var.db-name} user=${var.db-apiserver-user} password=${random_password.apiserver-password.result} connect_query='select 1'

[pgbouncer]
  listen_addr = *
  listen_port = 5432
  server_tls_sslmode = disable
  client_tls_sslmode = verify-full
  client_tls_ca_file = /config/root.crt
  client_tls_key_file = /config/pgbouncer.key
  client_tls_cert_file = /config/pgbouncer.crt
  client_tls_protocols = tlsv1.3
  auth_file = /config/pgbouncer-users.txt
  auth_type = cert
  ignore_startup_parameters = extra_float_digits
  pool_mode = transaction
  max_client_conn = 2000
  default_pool_size = 50
  min_pool_size = 10
EOF
    "pgbouncer-users.txt" =<<EOF
"${var.db-apiserver-user}" "${random_password.apiserver-password.result}"
EOF
  }
}

resource "tls_private_key" "root" {
  algorithm = "ECDSA"
  ecdsa_curve = "P256"
}
resource "tls_private_key" "apiserver" {
  algorithm = tls_private_key.root.algorithm
  ecdsa_curve = tls_private_key.root.ecdsa_curve
}
resource "tls_private_key" "pgbouncer" {
  algorithm = tls_private_key.root.algorithm
  ecdsa_curve = tls_private_key.root.ecdsa_curve
}

resource "tls_self_signed_cert" "root" {
  key_algorithm   = tls_private_key.root.algorithm
  private_key_pem = tls_private_key.root.private_key_pem

  validity_period_hours = 50000
  # Reasonable set of uses for a server SSL certificate.
  allowed_uses = [
      "cert_signing",
      "crl_signing",
      "content_commitment",
      "key_encipherement",
      "digital_signature",
  ]
  subject {
      common_name  = "geocube"
      organization = "GeoCube"
  }
  is_ca_certificate = true
}

resource "tls_cert_request" "apiserver" {
  key_algorithm   = tls_private_key.apiserver.algorithm
  private_key_pem = tls_private_key.apiserver.private_key_pem

  subject {
    common_name  = var.db-apiserver-user
    organization = "GeoCube"
  }
}

resource "tls_locally_signed_cert" "apiserver" {
  cert_request_pem   = tls_cert_request.apiserver.cert_request_pem
  ca_key_algorithm   = tls_private_key.root.algorithm
  ca_private_key_pem = tls_private_key.root.private_key_pem
  ca_cert_pem        = tls_self_signed_cert.root.cert_pem

  validity_period_hours = 50000

  allowed_uses = [
    "key_encipherment",
    "client_auth",
    "digital_signature",
  ]
}

resource "tls_cert_request" "pgbouncer" {
  key_algorithm   = tls_private_key.pgbouncer.algorithm
  private_key_pem = tls_private_key.pgbouncer.private_key_pem

  subject {
    common_name  = "pgbouncer"
    organization = "GeoCube"
  }
}

resource "tls_locally_signed_cert" "pgbouncer" {
  cert_request_pem   = tls_cert_request.pgbouncer.cert_request_pem
  ca_key_algorithm   = tls_private_key.root.algorithm
  ca_private_key_pem = tls_private_key.root.private_key_pem
  ca_cert_pem        = tls_self_signed_cert.root.cert_pem

  validity_period_hours = 50000

  allowed_uses = [
    "key_encipherment",
    "server_auth",
    "digital_signature",
  ]
}

resource "kubernetes_deployment" "pgbouncer" {
  metadata {
    name = "pgbouncer"
  }
  spec {
    replicas = 2
    selector {
      match_labels = {
        app = "pgbouncer"
      }
    }
    template {
      metadata {
        labels = {
          app = "pgbouncer"
        }
      }
      spec {
        container {
          name = "pgbouncer"
          image = "${var.registry}/${var.project-id}/pgbouncer:${var.short-sha}"
          image_pull_policy = "Always"
          command = ["/usr/bin/pgbouncer"]
          args = ["-u","nobody","/config/pgbouncer.ini"]
          port {
            container_port = 5432
          }
          volume_mount {
            mount_path = "/config"
            name = "config"
            read_only = true
          }
        }
        volume {
          name = "config"
          secret {
            secret_name = kubernetes_secret.pgbouncer.metadata[0].name
            default_mode = "0400"
          }
        }
      }
    }
  }
}

resource "kubernetes_service" "pgbouncer" {
  metadata {
    name = "pgbouncer"
  }
  spec {
    type = "LoadBalancer"
    load_balancer_ip = google_compute_address.geocube-address.address
    selector = {
        app = "pgbouncer"
    }
    port {
      protocol = "TCP"
      port = 5432
      target_port = 5432
    }
  }
}
*/


