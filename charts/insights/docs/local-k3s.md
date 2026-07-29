# Local k3s validation

Use [k3d](https://k3d.io/) to run a disposable single-node k3s cluster in
Docker. The chart also works with a native k3s installation.

## Prerequisites

- Docker
- k3d
- kubectl
- Helm 3 or 4

The container runtime needs at least 2 CPUs, 4 GiB of memory, and enough disk
space for the image and persistent volume.

## Create the cluster

```sh
k3d cluster create insights \
  --servers 1 \
  --agents 0 \
  --wait
kubectl config use-context k3d-insights
```

k3s includes the `local-path` default StorageClass, so the chart's PVC needs no
storage override.

## Install and validate Insights

From a checkout of this repository:

```sh
helm upgrade --install insights ./charts/insights \
  --namespace insights \
  --create-namespace \
  --set config.simulator.enabled=true \
  --set-string config.db.memory-limit=1GB \
  --set persistence.size=2Gi \
  --wait \
  --timeout 10m

kubectl get statefulset,pod,pvc,service -n insights
kubectl logs -n insights statefulset/insights
kubectl port-forward -n insights service/insights 8080:8080
```

Open <http://127.0.0.1:8080>. The simulator should populate the UI without an
external NATS deployment or license.

The default uses the unlicensed image without a pull Secret. If the registry
returns an authentication error, or your account requires authentication for
the selected repository, create one:

```sh
export SYNADIA_REGISTRY_USERNAME=your-username
export SYNADIA_REGISTRY_PASSWORD=your-password

kubectl create secret docker-registry synadia-registry \
  --namespace insights \
  --docker-server registry.synadia.io \
  --docker-username "$SYNADIA_REGISTRY_USERNAME" \
  --docker-password "$SYNADIA_REGISTRY_PASSWORD"
helm upgrade insights ./charts/insights \
  --namespace insights \
  --reuse-values \
  --set 'image.pullSecrets[0].name=synadia-registry'
```

Registry credentials and an Insights license are separate. Adding
`config.license.token` or `config.license.file` makes the chart select
`insights-licensed` automatically. See the chart README for secret-handling and
image-override options.

## Delete the cluster

```sh
k3d cluster delete insights
```
