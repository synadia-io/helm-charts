# Azure Kubernetes Service

This guide creates a one-node AKS cluster backed by an Azure managed disk.
Creating the cluster, node, and disk incurs Azure charges.

For production, use private networking as appropriate, restrict Kubernetes API
access, and ensure the cluster network can reach the NATS system being
monitored.

## Prerequisites

- Azure CLI authenticated to the target subscription
- kubectl
- Helm 3 or 4

## Create the cluster

```sh
AZURE_REGION=eastus
RESOURCE_GROUP=insights
CLUSTER_NAME=insights

az group create \
  --name "$RESOURCE_GROUP" \
  --location "$AZURE_REGION"

az aks create \
  --resource-group "$RESOURCE_GROUP" \
  --name "$CLUSTER_NAME" \
  --node-count 1 \
  --node-vm-size Standard_D2s_v5 \
  --enable-managed-identity \
  --generate-ssh-keys

az aks get-credentials \
  --resource-group "$RESOURCE_GROUP" \
  --name "$CLUSTER_NAME"

kubectl get nodes
kubectl get storageclass
```

AKS supplies `managed-csi`, backed by Azure Standard SSD, and
`managed-csi-premium`, backed by Azure Premium SSD. See the official
[AKS storage documentation](https://learn.microsoft.com/azure/aks/concepts-storage)
for current redundancy, reclaim, and expansion behavior.

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
  storageClass: managed-csi
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

Azure managed disks are `ReadWriteOnce`, which matches Insights' single-replica
StatefulSet. The chart-created PVC is retained after `helm uninstall`.

The official [AKS CLI quickstart](https://learn.microsoft.com/azure/aks/learn/quick-kubernetes-deploy-cli)
covers additional cluster configuration and operational considerations.

## Clean up

Deleting the resource group removes the AKS cluster and its Azure resources:

```sh
az group delete \
  --name "$RESOURCE_GROUP" \
  --yes
```
