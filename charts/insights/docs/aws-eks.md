# Amazon EKS

This guide creates an EKS Auto Mode cluster, which manages compute, networking,
and block storage. Creating the cluster, nodes, and EBS volumes incurs AWS
charges.

For production, place the cluster in private subnets, restrict Kubernetes API
access, and ensure its VPC can reach the NATS system being monitored.

## Prerequisites

- AWS CLI authenticated to the target account
- [eksctl](https://docs.aws.amazon.com/eks/latest/eksctl/what-is-eksctl.html)
- kubectl
- Helm 3 or 4

## Create the cluster

Create `eks-cluster.yaml`:

```yaml
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig
metadata:
  name: insights
  region: us-east-2

autoModeConfig:
  enabled: true
```

```sh
eksctl create cluster -f eks-cluster.yaml
kubectl get nodes
```

EKS Auto Mode provides the EBS CSI capability but does not create a
StorageClass. Create an encrypted `gp3` class:

```yaml
# storage-class.yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: auto-ebs-gp3
provisioner: ebs.csi.eks.amazonaws.com
volumeBindingMode: WaitForFirstConsumer
parameters:
  type: gp3
  encrypted: "true"
allowedTopologies:
  - matchLabelExpressions:
      - key: eks.amazonaws.com/compute-type
        values:
          - auto
```

```sh
kubectl apply -f storage-class.yaml
```

See the official [EKS Auto Mode](https://docs.aws.amazon.com/eks/latest/eksctl/auto-mode.html)
and [EBS StorageClass](https://docs.aws.amazon.com/eks/latest/userguide/create-storage-class.html)
documentation for production networking, IAM, encryption, and storage options.

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
  storageClass: auto-ebs-gp3
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

EBS volumes are `ReadWriteOnce` and bound to an Availability Zone, which matches
Insights' single-replica StatefulSet. The chart-created PVC is retained after
`helm uninstall`.

## Clean up

```sh
helm uninstall insights --namespace insights
kubectl delete pvc --namespace insights --all
eksctl delete cluster --name insights --region us-east-2
```
