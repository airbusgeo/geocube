# Installation on existing GKE Cluster, in other namespace with Terraform

In root folder `agri`

Cloud SQL must be activated in your GCP Project

Export **GOOGLE_APPLICATION_CREDENTIALS** environment variable (with necessary rights: Pub/Sub must be added even if IAM is Owner or editor)

Run `terraform init`

Run `terraform plan -out planfile`

- **var.namespace**
- Enter a value: namespace-test

- **var.project-id**
- Enter a value: d-gcb-geocuberd

- **var.registry**
- Enter a value: eu.gcr.io

- **var.short-sha**
- Enter a value: test
`

Then Run: `terraform apply "planfile"`