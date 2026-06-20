# Module 08 — Scaling & Resilience

Keeping your app available under load and during infrastructure changes.

## Horizontal Pod Autoscaler (HPA)

Automatically scales Deployment replicas based on metrics.

```yaml
# k8s/base/api/hpa.yaml
minReplicas: 2
maxReplicas: 6
metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

```bash
kubectl get hpa -n kubelab
kubectl describe hpa api-hpa -n kubelab
```

### Trigger Scaling

```bash
# Generate load
kubectl run loadgen --rm -it --restart=Never -n kubelab \
  --image=busybox -- sh -c \
  "while true; do wget -qO- http://api:8080/api/orders; done"

# Watch HPA scale up (in another terminal)
kubectl get hpa -n kubelab -w
kubectl get pods -n kubelab -w
```

Requires **metrics-server** (pre-installed on GKE).

## Pod Disruption Budget (PDB)

Ensures minimum availability during **voluntary** disruptions (node drains, cluster upgrades):

```yaml
# k8s/base/api/pdb.yaml
minAvailable: 1
```

```bash
kubectl get pdb -n kubelab
```

During a GKE node upgrade, Kubernetes respects PDB — won't evict too many API pods at once.

## Rolling Updates

```yaml
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 1        # 1 extra pod during update
    maxUnavailable: 0  # never go below desired count
```

Simulate an update:

```bash
kubectl set image deployment/api \
  api=REGION-docker.pkg.dev/PROJECT/kubelab/api:latest \
  -n kubelab

kubectl rollout status deployment/api -n kubelab
```

## Resource Requests & Limits

```yaml
resources:
  requests:
    cpu: 100m      # Guaranteed minimum
    memory: 128Mi
  limits:
    cpu: 500m      # Maximum allowed
    memory: 256Mi
```

- **Requests** — used for scheduling (which node has capacity?)
- **Limits** — OOMKill if memory exceeded; CPU throttled

```bash
kubectl top pods -n kubelab
```

## Jobs & CronJobs

**Job** — run once to completion (seed data):

```bash
kubectl get jobs -n kubelab
kubectl logs job/seed-data -n kubelab
```

**CronJob** — run on schedule (cleanup stale orders at 3 AM):

```bash
kubectl get cronjobs -n kubelab
kubectl describe cronjob cleanup-stale-orders -n kubelab
```

## Exercise

1. Manually scale: `kubectl scale deployment/api --replicas=5 -n kubelab`
2. Run load test and observe HPA
3. Scale back: `kubectl scale deployment/api --replicas=2 -n kubelab`
4. Cordon a node: `kubectl cordon NODE_NAME` then `kubectl drain NODE_NAME --ignore-daemonsets` — watch PDB in action
5. Uncordon: `kubectl uncordon NODE_NAME`

## Next

→ [Module 09: Observability](09-observability.md)
