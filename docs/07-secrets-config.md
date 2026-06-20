# Module 07 — Secrets & Config

Managing configuration and sensitive data in Kubernetes.

## ConfigMap — Non-Sensitive Config

```yaml
# k8s/base/config/configmap.yaml
data:
  DB_HOST: postgres
  DB_PORT: "5432"
  LOG_LEVEL: info
```

Consumed as environment variables:

```yaml
envFrom:
  - configMapRef:
      name: kubelab-config
```

Or mounted as files:

```yaml
volumeMounts:
  - name: config
    mountPath: /etc/config
volumes:
  - name: config
    configMap:
      name: kubelab-config
```

**Important:** Changing a ConfigMap does NOT automatically restart pods. Use a reloader or rollout restart:

```bash
kubectl rollout restart deployment/api -n kubelab
```

## Secret — Sensitive Data

```yaml
# k8s/base/config/secret.yaml
stringData:
  DB_PASSWORD: kubelab-change-me
```

Referenced individually:

```yaml
env:
  - name: DB_PASSWORD
    valueFrom:
      secretKeyRef:
        name: kubelab-secrets
        key: DB_PASSWORD
```

### Security Notes

- Secrets are base64-encoded, **not encrypted** by default in etcd
- GKE encrypts etcd at rest with Google-managed keys
- **Never commit real secrets to git**

```bash
# View secret (decoded)
kubectl get secret kubelab-secrets -n kubelab -o jsonpath='{.data.DB_PASSWORD}' | base64 -d
```

## Kustomize Config Generation

Overlays merge config without editing base files:

```yaml
# k8s/overlays/gke-prod/kustomization.yaml
configMapGenerator:
  - name: kubelab-config
    behavior: merge
    literals:
      - APP_ENV=production
      - LOG_LEVEL=warn
```

## Production Secret Management

For real deployments, replace plain Secrets with:

| Tool | How |
|------|-----|
| **GCP Secret Manager** | External Secrets Operator pulls secrets at runtime |
| **Sealed Secrets** | Encrypted secrets safe in git |
| **HashiCorp Vault** | Enterprise secret store |

Example with External Secrets (conceptual):

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: db-password
spec:
  secretStoreRef:
    name: gcp-secret-store
  target:
    name: kubelab-secrets
  data:
    - secretKey: DB_PASSWORD
      remoteRef:
        key: kubelab-db-password
```

## Exercise

1. Edit ConfigMap: `kubectl edit configmap kubelab-config -n kubelab` — change LOG_LEVEL
2. Restart API and verify env: `kubectl exec deployment/api -n kubelab -- env | grep LOG`
3. Create a secret imperatively: `kubectl create secret generic test -n kubelab --from-literal=key=value`
4. Explain why DB_PASSWORD is in Secret but DB_HOST is in ConfigMap

## Next

→ [Module 08: Scaling & Resilience](08-scaling-resilience.md)
