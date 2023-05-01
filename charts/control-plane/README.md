## Config Generation

The [config generation script](https://github.com/connecteverything/helix-alpha#config-generation) can do much of the heavy lifting to populate values for your Helix deployment.

### Chart Values

Details in the [values.yaml](values.yaml)

### Example

values.yaml
```
helix:
  config:
    public_url: "https://helix.example.com"
    nats_systems:
      - name: "nats-us-east-1"
        urls: "nats://nats-00.us-east-1a.example.com,nats://nats-01.us-east-1a.example.com"
        system_account_creds_file: "/conf/helix/nsc/nats-us-east-1/sys.creds"
        operator_signing_key_file: "/conf/helix/nsc/nats-us-east-1/operator.nk"
        
embeddedPostgres:
  persistent: true
  pvc:
    storageClassName: "gp3"
    size: "15Gi"

embeddedPrometheus:
  persistent: true
  pvc:
    storageClassName: "gp3"

ingress:
  enabled: true
  className: "nginx"
  hosts:
    - host: helix.example.com
      paths:
        - path: /
          pathType: ImplementationSpecific
  tls:
    - hosts:
        - helix.example.com
      secretName: helix-tls
```

values-secrets.yaml
```
imageCredentials:
  username: "synadia"
  password: "guest"

helix:
  secrets:
    nats_systems:
      nats-us-east-1:
        operator.nk: "k9yMHOqO1V8U24A+8ntyqY8lkvgo9fsGPaxUrQ0cDdgYcfZ2EAA7XdMAxvs6RY4C+c3zX0dYtJqBlvII=="
        sys.creds: "k8a8MyIcyQEofyF1W0hbn2BFFLh9236cSeXT2i4OggAndas5KRb5bI2doEtw9p03CPFr7o1ifaLCR6Vx..."
```

### Deploy the Helm Chart

```bash
helm repo add synadia https://connecteverything.github.io/helm-charts
helm repo update
helm upgrade --install helix -n helix --create-namespace -f values.yaml -f values-secrets.yaml synadia/helix
```

### Login Details

On first run, login credentials will be visible in the logs
```
kubectl logs -n helix deployment/helix
```

### Uninstall Chart and Purge Data
```
helm uninstall -n helix helix
```
