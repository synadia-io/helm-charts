# Synadia Private Link Helm Chart

## Accessing the Helm Chart

```bash
# add the synadia repo (only needs to be run once)
helm repo add synadia https://synadia-io.github.io/helm-charts

# update the synadia repo index (run to get updated chart versions)
helm repo update synadia

# now you can install the synadia/private-link chart
# note: you will need to configure image pull secrets for this to work
helm upgrade --install private-link synadia/private-link
```

### Useful Tools and References

- [Chart Values file](https://github.com/synadia-io/helm-charts/blob/main/charts/private-link/values.yaml) - lists all possible configuration options

## Common Configuration

### Image Pull Secret

By default, you must add an Image Pull Secret that allows you to pull the Private Link image:

```yaml
imagePullSecret:
  username: my-user
  password: my-password
```

### Full Example

**values.yaml**

```yaml
config:
  platformURL: https://cloud.synadia.com
  natsURL: nats://nats.nats.svc.cluster.local:4222
  token: agt_my_token
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
  private-link \
  synadia/private-link
```
