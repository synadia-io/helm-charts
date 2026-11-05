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

An in-place upgrade from chart 1.x to 2.x is not currently supported.
Control Plane v2 stores its data in a different format, and the upgrade would delete the 1.x PVCs.
The chart stops the upgrade when it finds a 1.x Deployment in the release.
Keep existing releases on 1.x with `--version 1.x`.

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

## Storage

Control Plane runs as a 1-replica StatefulSet.
It stores all of its state on one PVC, `data-<statefulset name>-0`, mounted at `/data`:

- the generated encryption key, when `config.kms.key` is not set
- the internal NATS JetStream data, which holds all Control Plane records
- the internal Prometheus data, when `config.dataSources.prometheus` is not set

The PVC size defaults to 20GiB and is set with `persistence.size`.
Kubernetes does not allow changes to the volume claim template after install, so resize the PVC directly.
`helm uninstall` does not delete the PVC.

HA deployments are not supported yet.
