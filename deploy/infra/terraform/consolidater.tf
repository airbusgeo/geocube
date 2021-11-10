resource "google_service_account_key" "consolidater" {
  service_account_id = google_service_account.consolidater.name
}

resource "kubernetes_secret" "consolidater-google-application-credentials" {
  type = "Opaque"
  metadata {
    name = "consolidater-google-application-credentials"
  }
  data = {
    "credentials.json" = base64decode(google_service_account_key.consolidater.private_key)
  }
}

resource "kubernetes_service_account" "autoscaler" {
  metadata {
    name = "autoscaler-sa"
  }
}

resource "kubernetes_role" "autoscaler" {
  metadata {
    name = "autoscaler-role"
  }
  rule {
    api_groups = [""]
    resources = ["pods"]
    verbs = ["get","list","delete","create"]
  }
  rule {
    api_groups = [""]
    resources = ["replicationcontrollers"]
    verbs = ["get"]
  }
}

resource "kubernetes_role_binding" "autoscaler" {
  metadata {
    name = "manage-autoscaled-pods"
  }
  subject {
    kind = "ServiceAccount"
    name = kubernetes_service_account.autoscaler.metadata[0].name
    namespace = "default"
  }
  role_ref {
    kind = "Role"
    name = kubernetes_role.autoscaler.metadata[0].name
    api_group = "rbac.authorization.k8s.io"
  }
}


resource "kubernetes_deployment" "consolidater-autoscaler" {
  metadata {
    name = "consolidater-autoscaler"
  }
  spec {
    replicas = 1
    selector {
      match_labels = {
        app = "consolidater-autoscaler"
      }
    }
    template {
      metadata {
        labels = {
          app = "consolidater-autoscaler"
        }
      }
      spec {
        automount_service_account_token = true
        service_account_name = kubernetes_service_account.autoscaler.metadata[0].name
        container {
          name = "autoscaler"
          image = "${var.registry}/${var.project-id}/autoscaler:${var.short-sha}"
          image_pull_policy = "Always"
          args = [
            "-update=30s",
            "-psSubscription=${google_pubsub_subscription.consolidations-worker.name}",
            "-project=${var.project-id}",
            "-rc=consolidater",
            "-ns=default",
            "-ratio=1",
            "-minratio=1",
            "-step=16",
            "-max=256",
            "-min=0",
            "-pod.cost.path=/termination_cost",
            "-pod.cost.port=9000"
          ]
          resources {
            requests = {
              cpu = "1m"
              memory = "30Mi"
            }
          }
        }
      }
    }
  }
}

resource "kubernetes_replication_controller" "consolidater" {
  metadata {
    name = "consolidater"
    labels = {
      app = "consolidater"
    }
  }
  lifecycle {
    ignore_changes = [spec[0].replicas]
  }
  spec {
    replicas = 0
    selector = {
        app = "consolidater"
    }
    template {
      metadata {
        labels = {
          app = "consolidater"
        }
      }
      spec {
        automount_service_account_token = true
        termination_grace_period_seconds = 5
        node_selector = {
          "cloud.google.com/gke-local-ssd" = "true"
        }
        toleration {
          key = "preemptible"
          operator = "Equal"
          value = "true"
          effect = "NoSchedule"
        }
        volume {
          name = "google-cloud-key"
          secret {
            optional = false
            secret_name = kubernetes_secret.consolidater-google-application-credentials.metadata[0].name
          }
        }
        volume {
          name = "local-ssd"
          host_path {
            path = "/mnt/disks/ssd0"
          }
        }
        container {
          name = "consolidater"
          image = "${var.registry}/${var.project-id}/consolidater:${var.short-sha}"
          //termination_message_policy = "FallbackToLogsOnError"
          image_pull_policy = "Always"
          port {
            container_port = 9000
            protocol = "TCP"
          }
          resources {
            requests = {
              cpu = "1900m"
              memory = "1500Mi"
            }
            limits = {
              memory = "4Gi"
            }
          }
          command = ["/bin/sh","-c"]
          args = [
          <<EOF
          UUID=`uuid`;
          WORKDIR=/local-ssd/$UUID;
          mkdir -p $WORKDIR;
          /consolidater -project ${var.project-id} -psEventTopic ${google_pubsub_topic.events.name} -psSubscription ${google_pubsub_subscription.consolidations-worker.name} -workdir $WORKDIR || true;
          exitcode=$?;
          rm -rf $WORKDIR;
          exit $exitcode;
          EOF
          ]
          env {
            name = "GOOGLE_APPLICATION_CREDENTIALS"
            value = "/var/secrets/google/credentials.json"
          }
          volume_mount {
            mount_path = "/local-ssd"
            name = "local-ssd"
          }
          volume_mount {
            mount_path = "/var/secrets/google"
            name = "google-cloud-key"
            read_only = true
          }
        }
      }
    }
  }
}


