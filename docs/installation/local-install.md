
# Installation - Local environment

## Environment of development

|   Name    	| Version 	|     link                               	  |
|:----------:	|:-------:	|:-----------------------------------------:|
|   Golang   	| >= 1.16 	|      https://golang.org/doc/install     	|
|    GDAL    	|  >= 3.3 	|             https://gdal.org            	|
|   Python   	|  >= 3.7 	|    https://www.python.org/downloads/    	|
| PostgreSQL 	|  >= 11  	|   https://www.postgresql.org/download/  	|
|   PostGIS  	|  >= 2.5 	|       https://postgis.net/install/      	|
|   Docker   	|    NC   	| https://docs.docker.com/engine/install/ 	|

## Clone repository

Clone `https://github.com/airbusgeo/geocube.git` repository.

## Build and run

### Apiserver

For more information concerning build and run go application, see: Build and run Go Application
https://golang.org/cmd/go/#hdr-Compile_packages_and_dependencies

In the geocube repository, `cmd/server`, run `go build` command in order to generate executable file:

```bash
$ cd cmd/server && go build
```

It creates the executable `server`. Some arguments are required in order to start the server.

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
    	storage where cancelled jobs are referenced. Must be reachable by the Consolidation Workers and the Geocube with read/write permissions
  -consolidationsQueue string
    	name of the pgqueue or the pubsub topic to send the consolidation orders
  -dbConnection string
    	database connection (ex: postgresql://user:password@localhost:5432/geocube)
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
  -eventsQueue string
    	name of the pgqueue or the pubsub topic to send the asynchronous job events
  -gdalBlockSize string
    	gdal blocksize value (default 1Mb) (default "1Mb")
  -gdalNumCachedBlocks int
    	gdal blockcache value (default 500) (default 500)
  -gdalStorageDebug
    	enable storage debug to use custom gdal storage strategy
  -ingestionStorage string
    	path to the storage where ingested and consolidated datasets will be stored. Must be reachable with read/write/delete permissions. (local/gs)
  -maxConnectionAge int
    	grpc max age connection
  -pgqConnection string
    	url of the postgres database to enable pgqueue messaging system (pgqueue only)
  -port string
    	geocube port to use (default "8080")
  -project string
    	project name (gcp only/not required in local usage)
  -tls
    	enable TLS protocol (certificate and key must be /tls/tls.crt and /tls/tls.key)
  -with-gcs
    	configure GDAL to use gcs storage (may need authentication)
  -with-s3
    	configure GDAL to use s3 storage (may need authentication)
  -workers int
    	number of parallel workers per catalog request (default 1)
```

Example (run):

```bash
$  ./server -ingestionStorage=/geocube-datasets -dbConnection=postgresql://user:password@localhost:5432/geocube -eventsQueue events -consolidationsQueue consolidations -cancelledJobs /tmp
{"severity":"info","timestamp":"2021-05-24T15:10:57.621+0200","message":"Geocube v0.3.0"}

```

### Consolidater

In the geocube repository, `cmd/consolidater`, run `go build` command in order to generate executable file:

```bash
$ cd cmd/consolidater && go build
```

It creates the executable `consolidater`. Some arguments are required in order to start a consolidation worker.

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
  -consolidationsQueue string
    	name of the messaging queue for consolidation jobs (pgqueue or pubsub subscription)
  -eventsQueue string
    	name of the messaging queue for job events (pgquue or pubsub topic)
  -gdalBlockSize string
    	gdal blocksize value (default 1Mb) (default "1Mb")
  -gdalNumCachedBlocks int
    	gdal blockcache value (default 500) (default 500)
  -gdalStorageDebug
    	enable storage debug to use custom gdal storage strategy
  -pgqConnection string
    	url of the postgres database to enable pgqueue messaging system (pgqueue only)
  -psProject string
    	subscription project (gcp pubSub only)
  -retryCount int
    	number of retries when consolidation job failed with a temporary error (default 1)
  -with-gcs
    	configure GDAL to use gcs storage (may need authentication)
  -with-s3
    	configure GDAL to use s3 storage (may need authentication)
  -workdir string
    	scratch work directory
  -workers int
    	number of workers for parallel tasks (default 1)
```

Example (run):

```bash
$  ./consolidater -workdir=/tmp -psProject geocube-emulator -eventsQueue events -consolidationsQueue consolidations -cancelledJobs /tmp
```

### Downloader 

Downloader service is useful if the server runs in a distant environment, and the local environment has an efficient access to the storage.

In the geocube repository, `cmd/downloader`, run `go build` command in order to generate executable file:

```bash
$ cd cmd/downloader && go build
```

It creates the executable `downloader`. Some arguments are required in order to start a downloader service.

Downloader available options:
```bash
$ ./downloader --help
Usage of ./downloader:
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
  -maxConnectionAge int
    	grpc max age connection
  -port string
    	geocube downloader port to use (default "8080")
  -tls
    	enable TLS protocol
  -with-gcs
    	configure GDAL to use gcs storage (may need authentication)
  -with-s3
    	configure GDAL to use s3 storage (may need authentication)
  -workers int
    	number of parallel workers per catalog request (default 1)
```

Basic example:

```shell
./downloader -port 8081 -workers 4
```

Example with GCS support:
```bash
./downloader -port 8081 -with-gcs -workers 4 -gdalBlockSize 2Mb
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


### Autoscaler

Autoscaling is not available in a local environment. See [K8S - Autoscaler](k8s-install.md#Autoscaler) for more information.

## Messaging Broker

### PGQueue

To use this messaging broker, create the `pgq_jobs` table in your postgres database using the following script `interface/messaging/pgqueue/create_table.sql`.

```bash
$ psql -h <database_host> -d <database_name> -f interface/messaging/pgqueue/create_table.sql
```

Then, start the apiserver and the consolidater with the corresponding arguments:
- `--pgqConnection`: connection uri to the postgres database (e.g. `postgresql://user:password@localhost:5432/geocube`)
- `--consolidationQueue consolidations`
- `--eventsQueue events`
And the Autoscaler, with:
- `--pgq-connection`: connection uri to the postgres database (e.g. `postgresql://user:password@localhost:5432/geocube`)
- `--queue consolidations`


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
