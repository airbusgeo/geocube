
# Installation - Kubernetes Cluster

## IAM & Security

All the notions of security and service account are not covered in this document. It is the responsibility of the installers.
The files presented below are available as examples/templates. They do not present any notions of security.

## Container Registry

You can create your own registry server: https://docs.docker.com/registry/deploying/ 

### Docker Hub

In case the images are stored on https://hub.docker.com, you can define them as follows in your kubernetes configuration files (postgresql example: `image: postgres:11`):

```yaml
apiVersion: networking.k8s.io/v1
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

In this example, [postgres:11](https://hub.docker.com/layers/postgres/library/postgres/11.0/images/sha256-05f9b83f85bdf0382b1cb8fb72d17d7c8098b0287d7dd1df4ff09aa417a0500b?context=explore) image will be loaded.

### Private Registry

You can configure your kubernetes deployment files with private docker registry.

For more information, see: [Pull an Image from a Private Registry](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/)

`imagePullSecrets` is defined in your kubernetes configuration files and image name is specified as follow ex: `image: geocube-private-image:tag`

## Database

Geocube server must have sufficient rights in order to read and write into database. For more information, see: [Client Authentication](https://www.postgresql.org/docs/11/auth-pg-hba-conf.html).

Geocube required that `max_connections` must be configured as `1024`.  For more information, see: [Server configuration](https://www.postgresql.org/docs/11/runtime-config-connection.html).

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
```yaml
containers:
  - args:
      - -dbName=geocube
      - -dbUser=apiserver
      - -dbPassword=mydbPassword
      - -dbHost=localhost:5432
      - -eventsQueue=events
      - -consolidationsQueue=consolidations
      - -ingestionStorage=/geocube-datasets or gs://my-bucket/geocube-datasets
      - -maxConnectionAge=3600
      - -workers=1
      - -cancelledJobs=/geocube-cancelled-jobs or gs://my-bucket/geocube-cancelled-jobs
    env:
      - name: PUBSUB_EMULATOR_HOST
        value: 0.0.0.0:8085
    image: registry/project/geocube-go-server:v1
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
```yaml
containers:
  - name: consolidater
    image: registry/project/consolidater:v1
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
        UUID=`uuidgen`;
        WORKDIR=/local-ssd/$UUID;
        mkdir -p $WORKDIR;
        /consolidater -eventsQueue events -consolidationsQueue consolidations -workdir $WORKDIR -cancelledJobs=/geocube-cancelled-jobs or gs://my-bucket/geocube-cancelled-jobs || true;
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
```yaml
containers:
  - name: autoscaler
    image: registry/project/autoscaler:v1
    imagePullPolicy: Always
    args:
      - -update=30s
      - -queue=consolidations
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
