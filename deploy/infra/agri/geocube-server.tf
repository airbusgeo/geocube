locals {
  k8s_api_env = {
    "REGION" = var.region
    "ZONE" = var.zone
  }
}



resource "kubernetes_secret" "apiserver-cert" {
  type = "Opaque"
  metadata {
    name = "apiserver-cert"
    namespace = kubernetes_namespace.geocube.metadata[0].name
  }
  data = {
    "tls.crt" = tls_locally_signed_cert.apiserver.cert_pem
    "tls.key" = tls_private_key.apiserver.private_key_pem
    "root.crt" = tls_self_signed_cert.root.cert_pem
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
    common_name  = "apiserver"
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

resource "google_service_account_key" "apiserver" {
  service_account_id = google_service_account.geocube-server.name
}

resource "kubernetes_secret" "apiserver-google-application-credentials" {
  type = "Opaque"
  metadata {
    name = "apiserver-google-application-credentials"
    namespace = kubernetes_namespace.geocube.metadata[0].name
  }
  data = {
    "credentials.json" = base64decode(google_service_account_key.apiserver.private_key)
  }
}

resource "kubernetes_deployment" "apiserver" {
  metadata {
    name = "apiserver"
    namespace = kubernetes_namespace.geocube.metadata[0].name
  }
  spec {
    replicas = 2
    selector {
      match_labels = {
        app = "apiserver"
      }
    }
    template {
      metadata {
        labels = {
          app = "apiserver"
        }
      }
      spec {
        automount_service_account_token = true
        volume {
          name = "google-cloud-key"
          secret {
            optional = false
            secret_name = kubernetes_secret.apiserver-google-application-credentials.metadata[0].name
          }
        }
        volume {
          name = "tls"
          secret {
            optional=false
            secret_name = kubernetes_secret.apiserver-cert.metadata[0].name
          }
        }
        container {
          name = "apiserver"
          image = "${var.registry}/${var.project-id}/geocube-go-server:${var.short-sha}"
          image_pull_policy = "Always"
          args = [
            "-project=${var.project-id}",
            "-dbName=${var.db-name}",
            "-dbUser=${google_sql_user.geocube-user.name}",
            "-dbPassword=$(DB_PASSWD)",
            "-dbHost=${google_sql_database_instance.instance.private_ip_address}",
            "-dbSecretName=${var.db-credentials-secret-name}",
            "-baSecretName=${var.bearer-auth-secret-name}",
            "-psEventsTopic=${google_pubsub_topic.events.name}",
            "-psConsolidationsTopic=${google_pubsub_topic.consolidations.name}",
            "-ingestionStorage=gs://${google_storage_bucket.geocube.name}",
            "-maxConnectionAge=3600"
          ]
          port {
            container_port = 8080
          }
          dynamic "env" {
            for_each = local.k8s_api_env
            content {
              name = env.key
              value = env.value
            }
          }
          env {
            name = "DB_PASSWD"
            value_from {
              secret_key_ref {
                name = kubernetes_secret.db.metadata.0.name
                key  = "db_apiserver_password"
              }
            }
          }
          env {
            name = "GOOGLE_APPLICATION_CREDENTIALS"
            value = "/var/secrets/google/credentials.json"
          }
          volume_mount {
            mount_path = "/var/secrets/google"
            name = "google-cloud-key"
            read_only = true
          }
          volume_mount {
            mount_path="/tls"
            name="tls"
            read_only=true
          }
        }
      }
    }
  }
}

resource "kubernetes_service" "apiserver" {
  metadata {
    name = "apiserver-service"
    annotations = {
      "cloud.google.com/app-protocols" = "{\"grpc\":\"HTTP2\"}"
      "beta.cloud.google.com/backend-config" = "{\"ports\": {\"grpc\":\"geocube-backendconfig\"}}"
      "cloud.google.com/backend-config" = "{\"ports\": {\"grpc\":\"geocube-backendconfig\"}}"
      "cloud.google.com/neg" = "{\"ingress\": true}"
    }
    namespace = kubernetes_namespace.geocube.metadata[0].name
  }
  spec {
    type = "NodePort"
    selector = {
        app = "apiserver"
    }
    port {
      name = "grpc"
      protocol = "TCP"
      port = 8080
      target_port = 8080
    }
  }
}

resource "google_compute_managed_ssl_certificate" "geocube" {
  provider = google-beta
  name = replace(var.endpoint-fqdn, ".","-")
  managed {
    domains = [var.endpoint-fqdn]
  }
}

resource "kubernetes_ingress" "default" {
  metadata {
    name = "apiserver-ingress"

    annotations = {
      "ingress.gcp.kubernetes.io/pre-shared-cert"   = google_compute_managed_ssl_certificate.geocube.name
      "kubernetes.io/ingress.global-static-ip-name" = google_compute_global_address.geocube-gw-address.name
    }

    namespace = kubernetes_namespace.geocube.metadata[0].name
  }

  spec {
    rule {
      http {
        path {
          backend {
            service_name = kubernetes_service.apiserver.metadata.0.name
            service_port = 8080
          }
        }
      }
    }
  }
}

