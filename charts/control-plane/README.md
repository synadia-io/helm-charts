# Synadia Control Plane Helm Chart

Chart version 2.x deploys Synadia Control Plane v2.
For Control Plane v1, use chart version 1.x.

## Accessing the Helm Chart

```bash
# add the synadia repo (only needs to be run once)
helm repo add synadia https://synadia-io.github.io/helm-charts

# update the synadia repo index (run to get updated chart versions)
helm repo update synadia

# now you can install the synadia/control-plane chart
# note: you will need to configure image pull secrets for this to work
helm upgrade --install control-plane synadia/control-plane
```

### Useful Tools and References

- [Chart Values file](https://github.com/synadia-io/helm-charts/blob/main/charts/control-plane/values.yaml) - lists all possible configuration options

## Upgrading from Chart 1.x

A multi-replica 1.x release (`singleReplicaMode.enabled: false`) upgrades in place to a multi-replica 2.x release.
On first start, Control Plane imports the 1.x data from the 1.x Postgres database.

1. Upgrade the release to the final 1.x chart version.
2. Keep the 1.x values file, and add `config.mode`.
   Every 1.x user logs in, and 2.x defaults to `standalone`, which has no login and makes every user an admin.
   To keep logins, set:

   ```yaml
   config:
     mode: managed
     merge:
       license:
         enabled: true
   ```

3. Run `helm upgrade` with the 2.x chart and the values file.

Chart 2.x reads these 1.x values:

| 1.x value | In 2.x |
|---|---|
| `config.dataSources.postgres` | Control Plane imports the 1.x data from this database on first start. |
| `config.kms` | Unchanged. It must include every active key URL that 1.x used, so that Control Plane can decrypt the 1.x data. |
| `deployment.replicas` | The number of replicas when `statefulSet.replicas` is not set. A cluster needs 3 or more. |
| `deployment.name` | The StatefulSet name when `statefulSet.name` is not set. |
| `deployment.merge`, `deployment.patch` | Not supported. Move them to `statefulSet` or `podTemplate`. |
| `container.image.tag` | A 1.x tag is not supported. Remove it to use the chart's default. |
| `singleReplicaMode.*`, `container.image.slim` | Not used. |

The 1.x Deployment is replaced by a StatefulSet.

A single-replica 1.x release cannot currently upgrade to 2.x.
Keep it on 1.x with `--version 1.x`.

Under `helm install` and `helm upgrade`, the chart stops an upgrade from 1.x that does not meet these requirements.
Under `helm template` and tools like Argo CD, the chart cannot read the cluster, so it does not check.

## Common Configuration

### Image Pull Secret

By default, you must add an Image Pull Secret that allows you to pull the Control Plane image:

```yaml
imagePullSecret:
  username: my-user
  password: my-password
```

### Mode

Control Plane runs in one of two modes:

- `standalone` (default) - there is no login, and every user is an admin.
  Do not expose a standalone Control Plane to untrusted networks.
- `managed` - users must log in.
  The first user to complete setup in the UI becomes the admin, so complete setup before you expose the UI.

```yaml
config:
  mode: managed
  merge:
    license:
      enabled: true
```

### Exposing Control Plane via Ingress

Control Plane web server can optionally be exposed via HTTP(S) using an Ingress.

```yaml
config:
  server:
    url: https://cp.nats.io

ingress:
  enabled: true
  className: nginx
  hosts:
    - cp.nats.io
  tlsSecretName: ingress-tls
```

### KMS Key

By default, Control Plane generates an encryption key and stores it on the data PVC.
To use your own key, set a KMS Key URL. URLs for KMS integrations are documented on the [GoCloud Secrets](https://gocloud.dev/howto/secrets/) website.
A `base64key://` URL holds 32 random bytes, encoded as URL-safe base64 with padding.
The below script will generate one, which can be used as the KMS Key URL:

```bash
echo "base64key://$(openssl rand -base64 32 | tr '+/' '-_')"
```

Set the URL directly, or store it in a Secret:

```bash
kubectl create secret generic control-plane-kms --from-literal=key.enc="base64key://..."
```

```yaml
config:
  kms:
    key:
      secretName: control-plane-kms
```

### Full Example

**values.yaml**

```yaml
config:
  mode: managed
  merge:
    license:
      enabled: true
  server:
    url: https://cp.nats.io

ingress:
  enabled: true
  className: nginx
  hosts:
    - cp.nats.io
  tlsSecretName: ingress-tls
```

**values-secrets.yaml**

```yaml
imagePullSecret:
  username: my-user
  password: my-password
```

**Deploy**

```bash
helm upgrade \
  --install \
  -f values.yaml \
  -f values-secrets.yaml \
  control-plane \
  synadia/control-plane
```

### Multi-Replica Cluster

Production deployments run a cluster of 3 or more replicas.
Every replica needs the same KMS key, and metrics go to an external Prometheus:

```yaml
statefulSet:
  replicas: 3

config:
  kms:
    key:
      secretName: control-plane-kms
  dataSources:
    prometheus:
      url: https://prometheus.example.com
```

A release cannot switch between a single replica and a multi-replica cluster after install, because Control Plane cannot convert its data between the two.
The chart stops such an upgrade under `helm upgrade`, and Control Plane refuses to start under other tools.
A multi-replica cluster can change its number of replicas.

A multi-replica cluster gets a PodDisruptionBudget that lets one replica be down at a time, so that a node drain does not break quorum.
Pods that are not ready can always be evicted, so a failing replica does not block a drain.
Set `podDisruptionBudget.enabled: false` to manage your own.

## Storage

Control Plane runs as a StatefulSet, with 1 replica by default.
Each replica stores all of its state on its own PVC, `data-<statefulset name>-<index>`, mounted at `/data`:

- the generated encryption key, when `config.kms.key` is not set
- the internal NATS JetStream data, which holds all Control Plane records
- the internal Prometheus data, when `config.dataSources.prometheus` is not set

The PVC size defaults to 20GiB and is set with `persistence.size`.
Kubernetes does not allow changes to the volume claim template after install, so resize the PVC directly.
`helm uninstall` and scaling down do not delete the PVCs.
