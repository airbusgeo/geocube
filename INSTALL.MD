# Geocube Installation Guide

## Table of Contents


- [Geocube Installation Guide](#geocube-installation-guide)
  - [Table of Contents](#table-of-contents)
  - [Prerequisites](#prerequisites)
- [Architecture](#architecture)
- [Interfaces](#interfaces)
  - [Storage](#storage)
    - [Interface](#interface)
    - [Currently supported storages](#currently-supported-storages)
  - [Messaging](#messaging)
    - [Interface](#interface-1)
    - [Pubsub implementation](#pubsub-implementation)
  - [Database](#database)
    - [Interface](#interface-2)
    - [PostgreSQL Implementation](#postgresql-implementation)
  - [Autoscaler](#autoscaler)
- [Installation - Local environment](#installation---local-environment)
  - [Environment of development](#environment-of-development)
  - [Build and run](#build-and-run)
    - [Apiserver](#apiserver)
    - [Consolidater](#consolidater)
  - [Messaging Broker](#messaging-broker)
    - [RabbitMq](#rabbitmq)
    - [Pub/Sub (Emulator)](#pubsub-emulator)
- [Installation - Docker](#installation---docker)
  - [Messaging Broker](#messaging-broker-1)
- [Installation - Kubernetes Cluster](#installation---kubernetes-cluster)
  - [IAM & Security](#iam--security)
  - [Container Registry](#container-registry)
    - [Docker Hub](#docker-hub)
    - [Private Registry](#private-registry)
  - [Database](#database-1)
  - [Pubsub Emulator](#pubsub-emulator-1)
  - [Apiserver](#apiserver-1)
  - [Consolidater](#consolidater-1)
  - [Reference](#reference)
    - [Kubernetes](#kubernetes)
- [Downloader](#downloader)
  - [Docker build](#docker-build)
  - [Docker run](#docker-run)
- [Geocube Upgrade](#geocube-upgrade)
  - [PostgreSQL database](#postgresql-database)


## Prerequisites

The Geocube needs:
- a Geospatial database (currently supported : Postgresql with PostGIS)
- a Messaging System to exchange messages between applications (currently supported: Pub/Sub)
- an Object Storage, writable and readable by range-request (currently supported: local storage or GCS)
- (Optional) a Scaling Platform to automatically scale the ressources (currently supported: K8S)

The Geocube can be run [locally](#installation---local-environment), as [dockers](#installation---docker) or deployed in a [cluster](#installation---kubernetes-cluster) (currently supported Kubernetes)

# Architecture

The Geocube Solution is composed of three parts:
- Geocube Service : to access and manage the geo data. It’s the purpose of this documentation.
- Geocube Ingester : to feed the Geocube with ARD images. An example of an ingester is available www.github.com/airbusgeo/geocube-ingester.
- Geocube Client: to use the geocube in an application. A [python client](www.github.com/airbusgeo/geocube-client-python) and a [go client](www.github.com/airbusgeo/geocube-client-go) are available as example. Clients (for another language) can be created from the [protobuf files](https://developers.google.com/protocol-buffers) using the automatically generated [GRPC](https://grpc.io/) interface (see [Tutorials](https://developers.google.com/protocol-buffers/docs/tutorials)).

The Geocube Server is composed of the ApiServer, the Consolidater and an interface layer to integrate into the user environment.

![Geocube Architecture](./docs/GeocubeArchitecture.png)

# Interfaces

To integrate into the environment of deployment, the geocube has an interface layer. Some implementations of this layer are available and the user is free to implement others depending on its own environment.

The interfaces are available in the `interface` folder.

## Storage

### Interface

The storage is used to read and write the images that are indexed in the Geocube. It must be accessible in reading by range-request and should be accessible in writing to support the consolidation (optimisation of the data).
The interface is available in `interface/storage` package.

### Currently supported storages

Currently, the geocube code supports two storage systems: GCS and filesystem.
To add a storage strategy, the following methods are to be implemented:

```golang
// Download file content as a slice of byte
Download(ctx context.Context, uri string, options ...Option) ([]byte, error)
// DownloadTo a local file
DownloadTo(ctx context.Context, source string, destination string, options ...Option) error
// Upload file content into remote file
Upload(ctx context.Context, uri string, data []byte, options ...Option) error
// UploadFile into remote file
UploadFile(ctx context.Context, uri string, data io.ReadCloser, options ...Option) error
// Delete file
Delete(ctx context.Context, uri string, options ...Option) error
// Exist checks if file exist
Exist(ctx context.Context, uri string) (bool, error)
// GetAttrs returns file attribute
GetAttrs(ctx context.Context, uri string) (Attrs, error)
```

The storage is infered from the prefix of the uri (protocol). The user can add an additionnal storage by implementing the interface and adding it in the `interface/storage/uri/` package.

## Messaging

### Interface

The messaging interface is available here : `interface/messaging/`.
It is used to communicate between the ApiServer and the Consolidater. It's a parameter of the constructor of the Service and it is configured in the following files: `cmd/apiserver/main.go` and `cmd/consolidater/main.go`.

### Pubsub implementation

Geocube supports PubSub (Google Cloud Platform) messaging broker : `vendor/github.com/geocube/interface/messaging/pubsub`.

Topics and subscriptions are to be created.

Topics:
- events
- events-failed
- consolidations
- consolidations-failed
- consolidations-worker (only if autoscaler is used)

Subscriptions:
- events
- consolidations
- consolidations-worker (only if autoscaler is used)

These actions could be performed manually or with terraform.
For more information, see: https://cloud.google.com/pubsub/docs/overview.
You must have the Pub/Sub Admin role on your service account.

NB: Topics & Subscriptions must be created before running services.

A Pub/Sub emulator is available to use PubSub in a local system (with limited capacities).

Please follow the [documentation](https://cloud.google.com/pubsub/docs/emulator) to install and start the emulator.


## Database

### Interface

The database interface is available here : `interface/database/db.go`.
It is used by the ApiServer as a parameter of the constructor of the service and it is configured in the following file: `cmd/apiserver/main.go`.

### PostgreSQL Implementation

Geocube currently supports a Postgresql database with the PostGIS extension (`interface/database/pg/`).
Create a database and run the installation SQL script in order to create all tables, schemas and roles.
This script is available in Geocube code source in `interface/database/pg/create.sql`

```bash
$ psql -h <database_host> -d <database_name> -f interface/database/pg/create.sql
```

For ugrade, see [Update PostgreSQL database](#postgresql-database)

## Autoscaler

The autoscaler handles the scale-up or down of the consolidator service.
It’s an external service and does not have an interface. The current implementation, using Kubernetes, is available here : `interface/autoscaler/` and it is used in the Autoscaler service : `cmd/autoscaler/main.go`

# Installation - Local environment

## Environment of development

|   Name    	| Version 	|     link                               	  |
|:----------:	|:-------:	|:-----------------------------------------:|
|   Golang   	| >= 1.13 	|      https://golang.org/doc/install     	|
|    GDAL    	|  >= 3.2 	|             https://gdal.org            	|
|   Python   	|  >= 3.7 	|    https://www.python.org/downloads/    	|
| PostgreSQL 	|  >= 11  	|   https://www.postgresql.org/download/  	|
|   PostGIS  	|  >= 2.5 	|       https://postgis.net/install/      	|
|   Docker   	|    NC   	| https://docs.docker.com/engine/install/ 	|

## Build and run

### Apiserver

For more information concerning build and run go application, see: Build and run Go Application
https://golang.org/cmd/go/#hdr-Compile_packages_and_dependencies

In Geocube Repository, run `go build` command in order to generate executable file:

```bash
$ cd cmd/server && go build
$ ls -l
-rw-rw-r-- 1 user user     2831 mai    7 15:34 auth.go
-rw-rw-r-- 1 user user    10762 mai   24 14:56 main.go
-rwxrwxr-x 1 user user 25267224 mai   24 14:57 server
```

Some arguments are required in order to run this file.

```bash
$ ./server --help
Usage of ./server:
  -aws-endpoint string
        define aws_endpoint for GDAL to use s3 storage (--with-s3)
  -aws-region string
        define aws_region for GDAL to use s3 storage (--with-s3)
  -aws-shared-credentials-file string
        define aws_shared_credentials_file for GDAL to use s3 storage (--with-s3)
  -baSecretName string
        name of the secret that stores the bearer authentication (admin & user) (gcp only)
  -cancelledJobs string
        storage where cancelled jobs are referenced
  -dbConnection string
        database connection (ex: postgresql://postgres:1234@localhost:5432/geocube)
  -dbHost string
        database host (see dbName)
  -dbName string
        database name (to connect with User, Host & Password)
  -dbPassword string
        database password (see dbName)
  -dbSecretName string
        name of the secret that stores credentials to connect to the database (gcp only)
  -dbUser string
        database user (see dbName)
  -gdalBlockSize string
        gdal blocksize value (default 1Mb) (default "1Mb")
  -gdalNumCachedBlocks int
        gdal blockcache value (default 500) (default 500)
  -gdalStorageDebug
        enable storage debug to use custom gdal storage strategy
  -ingestionStorage string
        path to the storage where ingested and consolidated datasets will be stored (local/gs)
  -local
        execute geocube in local environment
  -maxConnectionAge int
        grpc max age connection
  -port string
        geocube port to use (default "8080")
  -project string
        project name (gcp only/not required in local usage)
  -psConsolidationsTopic string
        name of the topic to send the consolidation orders (pubsub only)
  -psEventsTopic string
        name of the topic to send the consolidation events (pubsub only)
  -with-gcs
        configure GDAL to use gcs storage (may need authentication)
  -with-s3
        configure GDAL to use s3 storage (may need authentication)
  -workers int
        number of parallel workers per catalog request (default 1)
```

Example (run):

```bash
$  ./server -ingestionStorage=/geocube-datasets -dbConnection=postgresql://user:password@localhost:5432/geocube -psEventsTopic events -psConsolidationsTopic consolidations -local -cancelledJobs /tmp
{"severity":"info","timestamp":"2021-05-24T15:10:57.621+0200","message":"Geocube v0.3.0"}

```

### Consolidater

In Geocube Repository, run `go build` command in order to generate executable file:

```bash
$ cd cmd/consolidater && go build
$ ls -l
-rw-rw-r-- 1 user user    10762 mai   24 14:56 main.go
-rwxrwxr-x 1 user user 25267224 mai   24 14:57 consolidater
```

Some arguments are required in order to run this file.

```bash
$ ./consolidater --help
Usage of ./consolidater:
  -aws-endpoint string
        define aws_endpoint for GDAL to use s3 storage (--with-s3)
  -aws-region string
        define aws_region for GDAL to use s3 storage (--with-s3)
  -aws-shared-credentials-file string
        define aws_shared_credentials_file for GDAL to use s3 storage (--with-s3)
  -cancelledJobs string
        storage where cancelled jobs are referenced
  -gdalBlockSize string
        gdal blocksize value (default 1Mb) (default "1Mb")
  -gdalNumCachedBlocks int
        gdal blockcache value (default 500) (default 500)
  -gdalStorageDebug
        enable storage debug to use custom gdal storage strategy
  -psProject string
        subscription project (gcp pubSub only)
  -psConsolidationsSubscription string
        pubsub consolidation subscription name
  -psEventsTopic string
        pubsub events topic name
  -retryCount int
        number of retries when consolidation job failed with a temporary error (default 1)
  -workers int
        number of workers for parallel tasks        
  -with-gcs
        configure GDAL to use gcs storage (may need authentication)
  -with-s3
        configure GDAL to use s3 storage (may need authentication)
  -workdir string
        scratch work directory
```

Example (run):

```bash
$  ./consolidater -workdir=/tmp -psProject geocube-emulator -psEventsTopic events -psConsolidationsSubscription consolidations -cancelledJobs /tmp
```

## Messaging Broker

### RabbitMq

For more information, see: https://www.rabbitmq.com/#getstarted

You can easily run a rabbitmq application with docker. Run the following command:

```bash
$ docker run -d -p 5672:5672 -p 15672:15672 --name rabbit rabbitmq:3-management
```

NB: rabbitMq is not supported yet by Geocube.

### Pub/Sub (Emulator)

For more information, see: https://cloud.google.com/pubsub/docs/emulator

You can launch a local emulator with this command:

```bash
$ gcloud beta emulators pubsub start --project=geocube-emulator
Executing: /usr/lib/google-cloud-sdk/platform/pubsub-emulator/bin/cloud-pubsub-emulator --host=localhost --port=8085
[pubsub] This is the Google Pub/Sub fake.
[pubsub] Implementation may be incomplete or differ from the real system.
[pubsub] Jun 30, 2021 3:04:05 PM com.google.cloud.pubsub.testing.v1.Main main
[pubsub] INFO: IAM integration is disabled. IAM policy methods and ACL checks are not supported
[pubsub] SLF4J: Failed to load class "org.slf4j.impl.StaticLoggerBinder".
[pubsub] SLF4J: Defaulting to no-operation (NOP) logger implementation
[pubsub] SLF4J: See http://www.slf4j.org/codes.html#StaticLoggerBinder for further details.
[pubsub] Jun 30, 2021 3:04:06 PM com.google.cloud.pubsub.testing.v1.Main main
[pubsub] INFO: Server started, listening on 8085
```

Topics and subscription which are necessary for the proper functioning of the geocube, can be created by running the following script (replace `$GEOCUBE_SERVER` by the appropriate value):

```bash
$ go run tools/pubsub_emulator/main.go --project-id geocube-emulator --geocube-server https://$GEOCUBE_SERVER
2021/06/30 14:56:48 New client for project-id geocube-emulator
2021/06/30 14:56:48 Create Topic : consolidations
2021/06/30 14:56:48 Create Topic : consolidations-worker
2021/06/30 14:56:48 Create Topic : events
2021/06/30 14:56:48 Create Subscription : consolidations
2021/06/30 14:56:48 Create Subscription : consolidations-worker
2021/06/30 14:56:48 Create Subscription : events pushing to https://$GEOCUBE_SERVER/push
2021/06/30 14:56:48 Done!
```

In order to run geocube with the PubSub emulator, you must define the `PUBSUB_EMULATOR_HOST` environment variable (by default `localhost:8085`) **before** starting services.

# Installation - Docker

All dockerfile are available in the `docker` folder.

The Dockerfiles of the two services depend on a `base-image`:

```bash
$ docker build -f docker/Dockerfile.base-image -t geocube-base-image .
[...]
Successfully built 62eb9e6d2c0e
```

Then, the `BASE_IMAGE` must be passed as a parameter in order to build consolidater and server dockerfile:

```bash
$ docker build -f docker/Dockerfile.server -t geocube . --build-arg BASE_IMAGE=geocube-base-image
```

```bash
$ docker build -f docker/Dockerfile.consolidater -t geocube-consolidater . --build-arg BASE_IMAGE=geocube-base-image
```

You can run “docker run” command in order to start the application.

Examples :
```bash
export STORAGE=/geocube-datasets
docker run --rm --network=host -e PUBSUB_EMULATOR_HOST=localhost:8085 -v $STORAGE:$STORAGE geocube -project geocube-emulator -ingestionStorage=$STORAGE -dbConnection=postgresql://user:password@localhost:5432/geocube -psEventsTopic events -psConsolidationsTopic consolidations -local -cancelledJobs $STORAGE/cancelled-jobs
```
```bash
export STORAGE=/geocube-datasets
docker run --rm --network=host -e PUBSUB_EMULATOR_HOST=localhost:8085 -v $STORAGE:$STORAGE geocube-consolidater /consolidater -psProject geocube-emulator -workdir=/tmp -psEventsTopic events -psConsolidationsSubscription consolidations -cancelledJobs $STORAGE/cancelled-jobs
```

With GCS support (authentication with application_default_credentials.json):
```bash
export STORAGE=/geocube-datasets
docker run --rm -v ~/.config/gcloud:/root/.config/gcloud geocube -with-gcs [...]
```

For more information concerning running option, see: https://docs.docker.com/engine/reference/commandline/run/

## Messaging Broker

cf [Local environment - Messaging Broker](#messaging-broker)

# Installation - Kubernetes Cluster

## IAM & Security

All the notions of security and service account are not covered in this document. It is the responsibility of the installers.
The files presented below are available as examples/templates. They do not present any notions of security.

## Container Registry

You can create your own registry server: https://docs.docker.com/registry/deploying/ 

### Docker Hub

In case the images are stored on https://hub.docker.com, you can define them as follows in your kubernetes configuration files (postgresql example: `image: postgres:11`):

```kubernetes helm
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: postgresql
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: postgresql
          image: postgres:11
```

In this example, https://hub.docker.com/layers/postgres/library/postgres/11.0/images/sha256-05f9b83f85bdf0382b1cb8fb72d17d7c8098b0287d7dd1df4ff09aa417a0500b?context=explore image will be loaded.

### Private Registry

You can configure your kubernetes deployment files with private docker registry.

For more information, see: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/

`imagePullSecrets` is defined in your kubernetes configuration files and image name is specified as follow ex: `image: geocube-private-image:tag`

## Database

Geocube server must have sufficient rights in order to read and write into database. For more information, see: https://www.postgresql.org/docs/11/auth-pg-hba-conf.html

Geocube required that `max_connections` must be configured as `1024`.  For more information, see: https://www.postgresql.org/docs/11/runtime-config-connection.html

Kubernetes example configuration files are available in `deploy/k8s/database` in order to deploy minimal postgresql Database. All the parameters between `{{}}` are mandatory:
1. `{{POSTGRES_USER}}`: user name
2. `{{POSTGRES_PASSWORD}}`: user password

```bash
$ kubectl apply -f deploy/k8s/database/database.yaml
```

## Pubsub Emulator

Kubernetes configuration files are available in `deploy/k8s/pubSubEmulator` in order to deploy minimal pubSub emulator. `{{PUBSUB_EMULATOR_IMAGE}}` is to be defined (eg: `<container_registry>/pubsub-emulator:<tag>`)

```bash
$ kubectl apply -f deploy/k8s/pubSubEmulator/pubSub.yaml
```

You have to configure the access between PubSub and geocube’s components.

## Apiserver

Apiserver must have the necessary access to communicate with the database, the messaging service as well as the rights to read and write to the storage.

- Create apiserver service account

ApiServer must have suffisant rights in order to manage object storage and secrets access.

```bash
$ kubectl apply -f deploy/k8s/apiserver/service-account.yaml
```

- Create apiserver service

```bash
$ kubectl apply -f deploy/k8s/apiserver/service.yaml
```

- Create apiserver deployment

In order to start ApiServer, all the parameters between `{{}}` are to be defined in file `deploy/k8s/apiserver/deployment.yaml`:

1. `{{GEOCUBE_SERVER_IMAGE}}`: Geocube ApiServer Docker Image (eg. `<container_registry>/geocube-go-server:<tag>`)
2. Connection to the database `{{BD_HOST}}`, `{{DB_USER}}` and `{{DB_PASSWD}}`
3. `{{INGESTION_STORAGE}}`: uri to store ingested datasets (local and gcs uris are supported)
4. `{{PUBSUB_EMULATOR_HOST}}` environment variable can be added with pubSub emulator service IP (only if emulator is used)
5. `{{CANCELLED_JOBS_STORAGE}}`: uri to store cancelled jobs (local and gcs uris are supported)

Ex:
```kubernetes helm
containers:
  - args:
      - -dbName=geocube
      - -dbUser=apiserver
      - -dbPassword=mydbPassword
      - -dbHost=localhost:5432
      - -psEventsTopic=events
      - -psConsolidationsTopic=consolidations
      - -ingestionStorage=/geocube-datasets or gs://my-bucket/geocube-datasets
      - -maxConnectionAge=3600
      - -workers=1
      - -cancelledJobs=/geocube-cancelled-jobs or gs://my-bucket/geocube-cancelled-jobs
    env:
      - name: PUBSUB_EMULATOR_HOST
        value: 0.0.0.0:8085
    image: eu.gcr.io/project/geocube-go-server:v1
```

```bash
$ kubectl apply -f deploy/k8s/apiserver/deployment.yaml
```

## Consolidater

Consolidater must have the necessary access to communicate with the messaging service as well as the rights to read and write to the storage.

- Create Consolidater RoleBinding

```bash
$ kubectl apply -f deploy/k8s/consolidater/role-binding.yaml
```

- Create Consolidater Role (CRUD on pods & list on ReplicationControllers)

```bash
$ kubectl apply -f deploy/k8s/consolidater/role.yaml
```

- Create Autoscaler service account

```bash
$ kubectl apply -f deploy/k8s/consolidater/autoscaler-service-account.yaml
```

- Create Autoscaler replication controller

In order to start Autoscaler replication controller, you have to define some parameters in file `deploy/k8s/consolidater/replication-controller.yaml`:


1. `{{CONSOLIDATER_IMAGE}}`: Consolidater Docker Image (eg. `<container_registry>/consolidater:<tag>`).
2. `{{PUBSUB_EMULATOR_HOST}}` environment variable could be added with pubSub emulator service IP (only if emulator is used).
3. `{{CANCELLED_JOBS_STORAGE}}`: uri to store cancelled jobs (local and gcs uris are supported)

Ex:
```kubernetes helm
containers:
  - name: consolidater
    image: eu.gcr.io/project/consolidater:v1
    imagePullPolicy: "Always"
    ports:
      - containerPort: 9000
        protocol: TCP
    env:
      - name: PUBSUB_EMULATOR_HOST
        value: 0.0.0.0:8085       
[...]
    args:
      - |
        UUID=`uuid`;
        WORKDIR=/local-ssd/$UUID;
        mkdir -p $WORKDIR;
        /consolidater -psEventsTopic events -psConsolidationsSubscription consolidations -workdir $WORKDIR -cancelledJobs=/geocube-cancelled-jobs or gs://my-bucket/geocube-cancelled-jobs || true;
        exitcode=$?;
        rm -rf $WORKDIR;
        exit $exitcode;
```

```bash
$ kubectl apply -f deploy/k8s/consolidater/replication-controller.yaml
```

- Create autoscaler deployment

Define Autoscaler Docker Image `{{AUTOSCALER_IMAGE}}` (eg. `<container_registry>/autoscaler:<tag>`) in file `deploy/k8s/consolidater/autoscaler-deployment.yaml`

Ex:
```kubernetes helm
containers:
  - name: autoscaler
    image: eu.gcr.io/project/autoscaler:v1
    imagePullPolicy: Always
    args:
      - -update=30s
      - -psSubscription=consolidations
      - -rc=consolidater
      - -ns=default
      - -ratio=1
      - -minratio=1
      - -step=16
      - -max=256
      - -min=0
      - -pod.cost.path=/termination_cost
      - -pod.cost.port=9000
```

```bash
$ kubectl apply -f deploy/k8s/consolidater/autoscaler-deployment.yaml
```


## Reference

### Kubernetes

- **Deployment** describes a desired state of pod: https://kubernetes.io/docs/concepts/workloads/controllers/deployment
- **Pods** is a group of one or more containers: https://kubernetes.io/docs/concepts/workloads/pods
- **Secrets** lets you store and manage sensitive information, such as passwords, OAuth tokens, and ssh keys: https://kubernetes.io/docs/concepts/configuration/secret
- **Service** is an abstract way to expose an application running on a set of Pods as a network service: https://kubernetes.io/fr/docs/concepts/services-networking/service
- **Replication controller** ensures that a specified number of pod replicas are running at any one time: https://kubernetes.io/docs/concepts/workloads/controllers/replicationcontroller
- **RoleBinding** and **Role**: https://kubernetes.io/docs/reference/access-authn-authz/rbac/

# Downloader
The downloader is a standalone service to download cubes of data from metadata returned by a call to the GetCube function of a Geocube Server.

## Docker build
```shell
docker build -f docker/Dockerfile.downloader -t geocube-downloader . --build-arg BASE_IMAGE=geocube-base-image
```

## Docker run

Run locally with this command:
```shell
docker run --rm <image_to_run> <downloader_args>
```

Downloader available options:
```shell
	Usage of /downloader:
  -aws-endpoint string
        define aws_endpoint for GDAL to use s3 storage (--with-s3)
  -aws-region string
        define aws_region for GDAL to use s3 storage (--with-s3)
  -aws-shared-credentials-file string
        define aws_shared_credentials_file for GDAL to use s3 storage (--with-s3)
  -gdalBlockSize string
        gdal blocksize value (default 1Mb) (default "1Mb")
  -gdalNumCachedBlocks int
        gdal blockcache value (default 500) (default 500)
  -gdalStorageDebug
        enable storage debug to use custom gdal storage strategy
  -local
        execute geocube downloader in local environment
  -maxConnectionAge int
        grpc max age connection
  -port string
        geocube downloader port to use (default "8080")
  -with-gcs
        configure GDAL to use gcs storage (may need authentication)
  -with-s3
        configure GDAL to use s3 storage (may need authentication)
  -workers int
        number of parallel workers per catalog request (default 1)
```

Basic local example:

```shell
docker run --rm -p 127.0.0.1:8081:8081/tcp geocube-downloader -local -port 8081 -workers 4
```

Example with GCS support:

With GOOGLE_APPLICATION_CREDENTIALS:
```shell
docker run --rm -e GOOGLE_APPLICATION_CREDENTIALS=/account/geocube_server.json -p 127.0.0.1:8081:8081/tcp --mount type=bind,src=~/Documents/account/geocube,dst=/account 65cddc550e9a geocube-downloader -local -port 8081 -with-gcs -workers 4 -gdalBlockSize 2Mb
```

With application_default_credentials.json:
```bash
docker run --rm -v ~/.config/gcloud:/root/.config/gcloud -p 127.0.0.1:8081:8081/tcp geocube-downloader -local -port 8081 -with-gcs -workers 4 -gdalBlockSize 2Mb
```

Storage Debug (GCP only):

It's possible to monitor storage metrics with `--gdalStorageDebug` argument.

You will retrieve storage metrics into logs as:

```shell
{"severity":"debug","timestamp":"2022-01-14T11:29:15.618+0100","message":"GCS Metrics: gs://myBucket/32523_20m/2/-89/myLayout/myFile.tif - 2 calls - 2097152 octets"}
{"severity":"debug","timestamp":"2022-01-14T11:29:15.618+0100","message":"GCS Metrics: gs://myBucket/32523_20m/4/-90/myLayout/myFile.tif - 2 calls - 2097152 octets"}
{"severity":"debug","timestamp":"2022-01-14T11:29:15.618+0100","message":"GCS Metrics: gs://myBucket/32523_20m/5/-91/myLayout/myFile.tif - 2 calls - 2097152 octets"}
{"severity":"debug","timestamp":"2022-01-14T11:29:15.618+0100","message":"GCS Metrics: gs://myBucket/32523_20m/4/-91/myLayout/myFile.tif - 3 calls - 3145728 octets"}
{"severity":"debug","timestamp":"2022-01-14T11:29:15.618+0100","message":"GCS Metrics: gs://myBucket/32523_20m/5/-90/myLayout/myFile.tif - 2 calls - 2097152 octets"}
{"severity":"debug","timestamp":"2022-01-14T11:29:15.618+0100","message":"GCS Metrics: gs://myBucket/32523_20m/3/-89/myLayout/myFile.tif - 2 calls - 3145728 octets"}
{"severity":"debug","timestamp":"2022-01-14T11:29:15.618+0100","message":"GCS Metrics: gs://myBucket/32523_20m/2/-90/myLayout/myFile.tif - 2 calls - 2097152 octets"}
{"severity":"debug","timestamp":"2022-01-14T11:29:15.618+0100","message":"GCS Metrics: gs://myBucket/32523_20m/3/-90/myLayout/myFile.tif - 2 calls - 3145728 octets"}
{"severity":"debug","timestamp":"2022-01-14T11:29:15.618+0100","message":"GCS Metrics: gs://myBucket/32523_20m/6/-90/myLayout/myFile.tif - 2 calls - 2097152 octets"}
{"severity":"debug","timestamp":"2022-01-14T11:29:15.618+0100","message":"GCS Metrics: gs://myBucket/32523_20m/6/-91/myLayout/myFile.tif - 2 calls - 2097152 octets"}
{"severity":"debug","timestamp":"2022-01-14T11:29:15.618+0100","message":"GCS Metrics: gs://myBucket/32523_20m/3/-91/myLayout/myFile.tif - 3 calls - 3145728 octets"}

```

Logs: `https://docs.docker.com/engine/reference/commandline/logs/#examples`


# Geocube Upgrade
## PostgreSQL database
After upgrading the Geocube server, the database schema might need an update. Apply incrementally each `interface/database/pg/update_X.X.X.sql` with X.X.X corresponding to a Geocube Server version from your previous version to the current version.

```bash
$ psql -h <database_host> -d <database_name> -f interface/database/pg/update_X.X.X.sql
```