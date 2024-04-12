# Synadia Private Link Helm Chart

## Accessing the Helm Chart

```bash
# add the synadia repo (only needs to be run once)
helm repo add synadia https://synadia-io.github.io/helm-charts

# update the synadia repo index (run to get updated chart versions)
helm repo update synadia

# now you can install the synadia/private-link chart
helm upgrade --install private-link synadia/private-link
```

### Useful Tools and References

- [Chart Values file](https://github.com/synadia-io/helm-charts/blob/main/charts/private-link/values.yaml) - lists all possible configuration options

## Common Configuration

### Basic Example

```yaml
config:
  platformURL: https://cp.nats.io
  natsURL: nats://nats.nats.svc.cluster.local:4222
  token: agt_my_token
```

### Deploy on 3 separate hosts

```yaml
config:
  platformURL: https://cp.nats.io
  natsURL: nats://nats.nats.svc.cluster.local:4222
  token: agt_my_token

deployment:
  replicas: 3

podTemplate:
  topologySpreadConstraints:
    kubernetes.io/hostname:
      maxSkew: 1
      whenUnsatisfiable: DoNotSchedule
```

### Mutual TLS

```yaml
config:
  platformURL: https://cp.nats.io
  natsURL: nats://nats.nats.svc.cluster.local:4222
  token: agt_my_token
  tls:
    caCerts:
      enabled: true
      # example uses ConfigMap, but Secret is also supported with `secretName`
      configMapName: ca-certs
      # key in the ConfigMap or Secret that holds the PEM-encoded x509 CA Certificate list
      # defaults to `ca.crt`
      key: my-ca.crt
    clientCert:
      enabled: true
      secretName: private-link-tls
      # key in the Secret that holds the PEM-encoded x509 Client Certificate
      # defaults to tls.crt
      cert: my-tls.crt
      # key in the Secret that holds the PEM-encoded x509 Private Key
      # defaults to tls.key
      key: my-tls.key
```
