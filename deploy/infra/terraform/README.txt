manual steps before running:

- enable cloud build api and set correct iam permissions to cloudbuild service account

- docker registry:
	- if using other than eu.gcr.io:
		- update _REGISTRY in cloudbuild.yaml
		- modify gke_gcr_reader bucket name in gke.tf
	- push temp/empty image to eu.gcr.io/$PROJECT in order to in initialize bucket

- set up cloud build from gitlab:
  - create service account
  - create cloudbuild-$PROJECT bucket and grant "object admin" + "legacy bucket reader" to service account

- due to terraform limitations, the first run might need to be launched in two phases:
  - terraform apply -var="project-id=$PROJECT_ID" -var "short-sha=$SHORT_SHA"
        -target google_container_node_pool.preemptible
        -target google_cloud_run_service.apiserver-gw
        -target google_cloud_run_service.apiserver
        -target google_container_node_pool.main;
  - terraform apply -var="project-id=$PROJECT_ID" -var "short-sha=$SHORT_SHA"

- grant token creator role to pubsub sa:
  - see: https://cloud.google.com/pubsub/docs/push?authuser=1#authentication_and_authorization
  - PUBSUB_SERVICE_ACCOUNT="service-${PROJECT_NUMBER}@gcp-sa-pubsub.iam.gserviceaccount.com"
    gcloud projects add-iam-policy-binding ${PROJECT_ID} \
       --member="serviceAccount:${PUBSUB_SERVICE_ACCOUNT}"\
       --role='roles/iam.serviceAccountTokenCreator'
