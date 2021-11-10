# Installation

## Table of Contents
1. [Reference](#Reference)
2. [Using Terraform on Existing Cluster on GKE](#Using Terraform on Existing Cluster on GKE)
   * [Prerequisites](#Prerequisites)
   * [Install infra with terraform](#Install infra with terraform)
3. [Using Terraform on new Cluster on GKE](#Using Terraform on new Cluster on GKE)
   * [Prerequisites](#Prerequisites)
   * [Install infra with terraform](#Install infra with terraform)
4. [Using Terraform on new Cluster (other than GKE)](#Using Terraform on new Cluster (other than GKE))
   * [Database](#Database)
   * [Messaging](#Messaging)
   * [ApiServer](#ApiServer)
   * [Consolidater](#Consolidater)
5. [Projects](#Projects)
   * [Pull Geocube project](#Pull Geocube project)
   * [Push Geocube docker images on Container Registry](#Push Geocube docker images on Container Registry)

## Reference

* **Deployment** describes a desired state of pod: https://kubernetes.io/docs/concepts/workloads/controllers/deployment
* **Pods** is a group of one or more containers: https://kubernetes.io/docs/concepts/workloads/pods/
* **Secrets**  lets you store and manage sensitive information, such as passwords, OAuth tokens, and ssh keys: https://kubernetes.io/docs/concepts/configuration/secret/
* **Service** is an abstract way to expose an application running on a set of Pods as a network service: https://kubernetes.io/fr/docs/concepts/services-networking/service/
* **Replication controller** ensures that a specified number of pod replicas are running at any one time: https://kubernetes.io/docs/concepts/workloads/controllers/replicationcontroller/
* **RoleBinding & Role** https://kubernetes.io/docs/reference/access-authn-authz/rbac/


## Using Terraform on Existing Cluster on GKE (Example AGRI)

### Prerequisites

* gcloud: https://cloud.google.com/sdk/docs/install
* kubectl: https://kubernetes.io/fr/docs/tasks/tools/install-kubectl/
* Terraform: https://www.terraform.io/downloads.html

Make sure you gcloud is configured to right Google Cloud platform project: https://cloud.google.com/sdk/gcloud/reference/config/set

NB: On GCP, create database and Pub/Sub dependencies with Terraform or manually.

### Install infra with terraform

In the **agri** package you can find the Terraform configuration files which allow to create all the resources except that of Cluster.

Run `export GOOGLE_APPLICATION_CREDENTIALS={{your_service_account}}`

Run `cd deploy/infra/agri`

Run `terraform init -backend-config=bucket={{tf-config-bucket}}} -backend-config=prefix=dev;`

Run `terraform plan -out planfile -var="project-id={PROJECT_ID}";`

Run `terraform apply planfile;`

## Using Terraform on new Cluster on GKE

### Prerequisites

* gcloud: https://cloud.google.com/sdk/docs/install
* kubectl: https://kubernetes.io/fr/docs/tasks/tools/install-kubectl/
* Terraform: https://www.terraform.io/downloads.html

Make sure you gcloud is configured to right Google Cloud platform project: https://cloud.google.com/sdk/gcloud/reference/config/set

NB: On GCP, create database and Pub/Sub dependencies with Terraform or manually.

### Install infra with terraform

In the **terraform** package you can find the Terraform configuration files which allow to create all the resources.

Run `export GOOGLE_APPLICATION_CREDENTIALS={{your_service_account}}`

Run `cd deploy/infra/terraform`

Run `terraform init -backend-config=bucket={{tf-config-bucket}}} -backend-config=prefix=dev;`

Run `terraform plan -out planfile -var="project-id={PROJECT_ID}";`

Run `terraform apply planfile;`

## Using Terraform on new Cluster (other than GKE)

### Database 

PostreSQL 11 with HSTORE, POSTGIS supports is required.

* PostreSQL: https://www.postgresql.org/download/
* PostGIS: https://postgis.net/install/

A geocube database must be create.
After that, run sql script in order to create all tables, schemas and roles.

You can find example, in order to deploy postgrsesql on kubernetes cluster in **deploy/k8s/database** folder.

`psql -h <database_host> -d <database_name> -f interface/database/pg/create_0.1.0.sql`

### Messaging

On GCP, Pub/Sub is used

Create `events`, `events-failed`,`consolidations` and `consolidationq-failed` topics with `allowed_persistence_regions` `message_storage_policy` configuration.

Create `events` and `consolidations` subscriptions.

You can find example, in order to deploy emulator on kubernetes cluster in **deploy/k8s/pubSubEmulator** folder.

Golang script is also available in folder **tools/pubsub_emulator**.

### ApiServer

All files are in **deploy/k8s/apiserver** folder.

### Consolidater

All files are in **deploy/k8s/consolidater** folder.

## Projects

### Pull Geocube project

Run `git clone git@code.webfactory.intelligence-airbusds.com:geocube/geocube.git`

Run `cd geocube/`

### Push Geocube docker images on Container Registry

Make sure you have permissions to create bucket.

Create Bucket `{PROJECT_ID}_cloudbuild` and grant `object admin` + `legacy bucket reader` to service account

Make sure you have permissions to upload images on Container Registry.

Make sure you gcloud is configured to right Google Cloud platform project: https://cloud.google.com/sdk/gcloud/reference/config/set

Run `gcloud builds submit --config tools/image_build_registry/cloudbuildimage.yaml --gcs-log-dir=gs://{PROJECT_ID}_cloudbuild/logs --gcs-source-staging-dir=gs://{PROJECT_ID}_cloudbuild/source .`


