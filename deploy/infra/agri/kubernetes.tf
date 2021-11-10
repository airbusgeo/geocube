resource "kubernetes_namespace" "geocube" {
  metadata {
    annotations = {
      name = var.namespace
    }

    labels = {
      mylabel = var.namespace
    }

    name = var.namespace
  }
}