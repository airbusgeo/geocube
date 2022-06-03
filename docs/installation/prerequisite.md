# Prerequisites

The Geocube needs:

- a Geospatial database (currently supported : Postgresql with PostGIS)
- a Messaging System to exchange messages between applications (currently supported: [Pub/Sub](https://cloud.google.com/pubsub/docs/overview) and Postgresql using [pgqueue](https://github.com/btubbs/pgq))
- an Object Storage, writable and readable by range-request (currently supported: local storage, [Google Storage](https://cloud.google.com/storage), [AWS-S3](https://aws.amazon.com/s3/))
- (Optional) a Scaling Platform to automatically scale the ressources (currently supported: [Kubernetes](https://kubernetes.io/))

The Geocube can be run [locally](local-install.md), as [dockers](docker-install.md) or deployed in a [cluster](k8s-install.md).
