Installation de Telepresence
https://www.telepresence.io/reference/install.html

** Création Deployment telepresence dans le cluster (attention namespace à modifier)

`kubectl apply -f telepresence.yaml`

** Connexion à telepresence

`telepresence --deployment telepresence --run-shell --also-proxy <ID_PRIVATE_BDD>`

NB: you need to be connect on your cluster with gcloud command :

`gcloud container clusters get-credentials $(CLUSTER_NAME) --zone $(CLUSTER_ZONE) --project $(PROJECT_NAME)`

** Connexion à la base de données avec PSQL (Connexion possible avec PGADMIN)

`psql -h <ID_PRIVATE_BDD> -p 5432 -d geocube -U geocube-root`

** Exécution du SCRIPT (possible directement dans PGADMIN)

`psql -h <ID_PRIVATE_BDD> -p 5432 -d geocube -U geocube-root -f create_0.1.0.sql`

Remarque:

Récupération du mot de passe

`kubectl get secret --namespace <NAMESPACE> db -o 'go-template={{index .data "db_root_password"}}'|base64 -d`


Suppression telepresence du cluster
`kubectl delete -f telepresence.yaml`
