# Synadia Control Plane

## Config Generation

The [config generation script](https://github.com/ConnectEverything/control-plane-beta#config-generation) can do much of the heavy lifting to populate values for your Synadia Control Plane deployment.

### Chart Values

Details in [values.yaml](values.yaml)

### Example

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

### Deploy the Helm Chart

```bash
helm repo add synadia https://connecteverything.github.io/helm-charts
helm repo update
helm upgrade --install control-plane -n syn-cp --create-namespace -f values.yaml -f values-secrets.yaml synadia/control-plane
```

### Login Details

On first run, login credentials will be visible in the logs

```bash
kubectl logs -n syn-cp deployment/control-plane
```

### Uninstall Chart and Purge Data

```bash
helm uninstall -n syn-cp control-plane
```
