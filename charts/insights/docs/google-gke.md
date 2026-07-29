# Google Kubernetes Engine

This guide creates a GKE Autopilot cluster and uses a Compute Engine persistent
disk. Creating the cluster workloads and disk incurs Google Cloud charges.

For production, use private networking as appropriate, restrict Kubernetes API
access, and ensure the VPC can reach the NATS system being monitored.

## Prerequisites

- Google Cloud CLI authenticated to the target project
- kubectl
- Helm 3 or 4

## Create the cluster

```sh
GCP_PROJECT=your-project-id
GCP_REGION=us-central1
CLUSTER_NAME=insights

gcloud config set project "$GCP_PROJECT"
gcloud services enable container.googleapis.com
gcloud container clusters create-auto "$CLUSTER_NAME" \
  --region "$GCP_REGION"
gcloud container clusters get-credentials "$CLUSTER_NAME" \
  --region "$GCP_REGION"

kubectl get nodes
kubectl get storageclass
```

GKE supplies the `standard-rwo` StorageClass backed by a balanced persistent
disk. See the official
[GKE persistent-volume documentation](https://cloud.google.com/kubernetes-engine/docs/concepts/persistent-volumes)
for disk types, topology, reclaim behavior, and backups.

## Install Insights

The default uses the unlicensed image without a pull Secret. If the registry
returns an authentication error, or your account requires authentication for
the selected repository, authenticate the namespace:

```sh
export SYNADIA_REGISTRY_USERNAME=your-username
export SYNADIA_REGISTRY_PASSWORD=your-password

kubectl create namespace insights
kubectl create secret docker-registry synadia-registry \
  --namespace insights \
  --docker-server registry.synadia.io \
  --docker-username "$SYNADIA_REGISTRY_USERNAME" \
  --docker-password "$SYNADIA_REGISTRY_PASSWORD"
```

If you create it, add the following to `insights-values.yaml`:

```yaml
image:
  pullSecrets:
    - name: synadia-registry
```

Create `insights-values.yaml`:

```yaml
config:
  data-dir: /var/lib/insights
  web:
    hostname: 0.0.0.0
  simulator:
    enabled: true
  db:
    memory-limit: 1GB

persistence:
  storageClass: standard-rwo
  size: 20Gi

resources:
  requests:
    cpu: 500m
    memory: 1Gi
  limits:
    memory: 2Gi
```

```sh
helm repo add synadia https://synadia-io.github.io/helm-charts
helm repo update synadia
helm upgrade --install insights synadia/insights \
  --namespace insights \
  --create-namespace \
  --values insights-values.yaml \
  --wait \
  --timeout 15m

kubectl get statefulset,pod,pvc,service -n insights
kubectl port-forward -n insights service/insights 8080:8080
```

Replace `simulator` with `sys` configuration to monitor a real NATS system.
Follow the chart's Secret guidance rather than placing production credentials
directly in a Helm values file.

The resource settings above are simulator-sized. For a real system, size
`config.db.memory-limit` and the pod memory request and limit together, leaving
headroom for Go, embedded NATS, and DuckDB allocations outside its buffer pool.

Registry credentials and an Insights license are separate. Adding
`config.license.token` or `config.license.file` makes the chart select
`insights-licensed` automatically. See the chart README for secret-handling and
image-override options.

Compute Engine persistent disks are `ReadWriteOnce`, which matches Insights'
single-replica StatefulSet. The chart-created PVC is retained after
`helm uninstall`.

See the official
[GKE Autopilot cluster guide](https://cloud.google.com/kubernetes-engine/docs/how-to/creating-an-autopilot-cluster)
for production cluster configuration.

## Clean up

```sh
helm uninstall insights --namespace insights
kubectl delete pvc --namespace insights --all
gcloud container clusters delete "$CLUSTER_NAME" \
  --region "$GCP_REGION" \
  --quiet
```
