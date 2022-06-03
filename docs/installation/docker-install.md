
# Installation - Docker

## Clone repository

Clone `https://github.com/airbusgeo/geocube.git` repository.


All dockerfile are available in the `docker` folder.

## Base-image
The Dockerfiles of the other services depend on a `base-image`:

```bash
$ docker build -f docker/Dockerfile.base-image -t geocube-base-image .
[...]
Successfully built 62eb9e6d2c0e
```

## Server, Consolidater and Downloader

Then, the `BASE_IMAGE` must be passed as a parameter in order to build server, consolidater or downloader dockerfile:

```bash
$ docker build -f docker/Dockerfile.server -t geocube . --build-arg BASE_IMAGE=geocube-base-image
```

```bash
$ docker build -f docker/Dockerfile.consolidater -t geocube-consolidater . --build-arg BASE_IMAGE=geocube-base-image
```

```bash
$ docker build -f docker/Dockerfile.downloader -t geocube-downloader . --build-arg BASE_IMAGE=geocube-base-image
```

You can run “docker run” command in order to start the application.

### Run server - examples:
```bash
export STORAGE=/geocube-datasets
docker run --rm --network=host -e PUBSUB_EMULATOR_HOST=localhost:8085 -v $STORAGE:$STORAGE geocube -project geocube-emulator -ingestionStorage=$STORAGE -dbConnection=postgresql://user:password@localhost:5432/geocube -eventsQueue events -consolidationsQueue consolidations -cancelledJobs $STORAGE/cancelled-jobs
```
```bash
export STORAGE=/geocube-datasets
docker run --rm --network=host -e PUBSUB_EMULATOR_HOST=localhost:8085 -v $STORAGE:$STORAGE geocube-consolidater /consolidater -psProject geocube-emulator -workdir=/tmp -eventsQueue events -consolidationsQueue consolidations -cancelledJobs $STORAGE/cancelled-jobs
```

With GCS support (authentication with application_default_credentials.json):
```bash
export STORAGE=/geocube-datasets
docker run --rm -v ~/.config/gcloud:/root/.config/gcloud geocube -with-gcs [...]
```

For more information concerning running option, see: https://docs.docker.com/engine/reference/commandline/run/

### Run downloader - examples:

Basic example:
```shell
docker run --rm -p 127.0.0.1:8081:8081/tcp geocube-downloader -port 8081 -workers 4
```

Example with GCS support:

With GOOGLE_APPLICATION_CREDENTIALS:
```shell
docker run --rm -e GOOGLE_APPLICATION_CREDENTIALS=/account/geocube_server.json -p 127.0.0.1:8081:8081/tcp --mount type=bind,src=~/Documents/account/geocube,dst=/account 65cddc550e9a geocube-downloader -port 8081 -with-gcs -workers 4 -gdalBlockSize 2Mb
```

With application_default_credentials.json:
```bash
docker run --rm -v ~/.config/gcloud:/root/.config/gcloud -p 127.0.0.1:8081:8081/tcp geocube-downloader -port 8081 -with-gcs -workers 4 -gdalBlockSize 2Mb
```

Additionnal information [here](local-install.md#downloader)


## Messaging Broker

cf [Local environment - Messaging Broker](local-install.md#messaging-broker)

## Docker-compose

A docker-compose file is provided as example. It's a minimal example, so feel free to edit it to take advantage of the full power of the Geocube.

- Copy the `./docker/.env.example` to `./docker/.env`
- Edit `./docker/.env` to set the `STORAGE_URI` (it will be mount as a volume to access and store images).
- Build the [base image](#base-image)
- `cd docker` and `docker-compose up`

