# Synadia Control Plane Helm Chart

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

- [Config Generation Script](https://github.com/synadia-io/control-plane-beta#config-generation) - can do much of the heavy lifting to populate values for your Synadia Control Plane deployment
- [Chart Values file](https://github.com/synadia-io/helm-charts/blob/main/charts/control-plane/values.yaml) - lists all possible configuration options
- Login Details - On first run, the `admin` user's credentials will be printed to the logs here:
  ```bash
  kubectl logs -c syn-cp deployment/control-plane
  ```

## Common Configuration

### Image Pull Secret

By default, you must add an Image Pull Secret that allows you to pull the Control Plane image:

```yaml
imagePullSecret:
  username: my-user
  password: my-password
```

### Configuring NATS Systems

NATS Systems are JWT-operator enabled NATS Clusters that you want to manage using Control Plane.  The System URL, A System Account User Creds file, and an Operator Signing key are all required.

```yaml
config:
  systems:
    # key is the Name of the System, once it is deployed the Name here should not be changed
    MySystem:
      url: nats://demo.nats.io
      systemUserCreds:
        contents: |
          paste system account user creds file contents here
      operatorSigningKey:
        contents: |
          paste operator signing key here
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
  - host: cp.nats.io
  tlsSecretName: ingress-tls
```

### Full Example

**values.yaml**

```yaml
config:
  server:
    url: https://cp.nats.io
  systems:
    MySystem:
      url: nats://demo.nats.io

ingress:
  enabled: true
  className: nginx
  hosts:
  - host: cp.nats.io
  tlsSecretName: ingress-tls
```

**values-secrets.yaml**

```yaml
imagePullSecret:
  username: my-user
  password: my-password

config:
  systems:
    MySystem:
      systemUserCreds:
        contents: |
          paste system account user creds file contents here
      operatorSigningKey:
        contents: |
          paste operator signing key here
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

## Deployment Modes

### Single Replica Deployment

By default, the Control Plane deployment is a 1-replica Deployment that will create 3 Persistent Volume Claims.

1. 1GiB PVC mounted at `/data/encryption` - stores the auto-generated KMS key.
   This PVC is not needed if `config.kms.key` is configured.
2. 10GiB PVC mounted at `/data/postgres` - stores the internal PostgreSQL Database data.
   This PVC is not needed if an external PostgreSQL Database at `config.dataSources.postgres` is configured.
3. 10GiB PVC mounted at `/data/prometheus` - stores the internal Prometheus Server data.
   This PVC is not needed if an external Prometheus Server at `config.dataSources.prometheus` is configured.

### HA Deployment

Requirements for an HA Deployment:

1. KMS Key URL.  URLs for KMS integrations are documented on the [GoCloud Secrets](https://gocloud.dev/howto/secrets/) website. 
   This script will generate a random base64 key, which can be used as the KMS Key URL:
   ```bash
   echo "base64key://$(head -c 32 /dev/urandom | base64)"
   ```
2. External PostgreSQL Database
3. External Prometheus Server

Example Configuration:

```yaml
config:
  kms:
    key:
      url: your KMS Key URL
  dataSources:
    postgres:
      dsn: your PostgreSQL DSN
    prometheus:
      url: your Prometheus URL

deployment:
  replicas: 2

singleReplicaMode:
  enabled: false
```
