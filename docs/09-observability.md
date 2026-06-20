# Module 09 — Observability

Logs, metrics, and health — knowing what's happening in your cluster.

## Logs

### kubectl logs

```bash
# Single pod
kubectl logs -l app.kubernetes.io/name=api -n kubelab -f

# Previous crashed container
kubectl logs POD_NAME -n kubelab --previous

# All containers in pod
kubectl logs POD_NAME -n kubelab --all-containers
```

### GKE Cloud Logging

GKE automatically ships container logs to **Cloud Logging**:

1. GCP Console → Logging → Logs Explorer
2. Filter: `resource.type="k8s_container" resource.labels.namespace_name="kubelab"`

Our API emits structured JSON logs (Go `slog`) — easy to query:

```
jsonPayload.msg="order processed"
```

## Metrics

### Prometheus Endpoint

Our API exposes `/metrics` in Prometheus format:

```bash
kubectl port-forward svc/api -n kubelab 8080:8080
curl localhost:8080/metrics
```

Key metrics:
- `kubelab_orders_created_total`
- `kubelab_http_requests_total`
- Go runtime metrics

Deployment annotations enable scraping:

```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8080"
  prometheus.io/path: "/metrics"
```

### GKE Managed Prometheus

Enable in GCP Console → Kubernetes Engine → Features → Managed Service for Prometheus.

Or use **Google Cloud Monitoring** dashboards for CPU, memory, pod count.

### kubectl top

```bash
kubectl top nodes
kubectl top pods -n kubelab
```

## Health Checks

| Endpoint | Purpose | Used by |
|----------|---------|---------|
| `/health` | Process alive | Liveness probe, LB |
| `/ready` | Dependencies OK | Readiness probe |

Test readiness failure — scale postgres to 0:

```bash
kubectl scale statefulset/postgres --replicas=0 -n kubelab
kubectl get pods -l app.kubernetes.io/name=api -n kubelab -w
# API pods go Not Ready (removed from Service endpoints)
kubectl scale statefulset/postgres --replicas=1 -n kubelab
```

## Events

```bash
kubectl get events -n kubelab --sort-by='.lastTimestamp'
```

Events show: scheduling, pulling images, probe failures, scaling.

## Debugging Checklist

When something breaks:

```bash
# 1. Pod status
kubectl get pods -n kubelab

# 2. Why is it failing?
kubectl describe pod POD_NAME -n kubelab

# 3. Application logs
kubectl logs POD_NAME -n kubelab

# 4. Recent events
kubectl get events -n kubelab --field-selector involvedObject.name=POD_NAME

# 5. Can you reach it internally?
kubectl run curl --rm -it --image=curlimages/curl -n kubelab -- \
  curl -v http://api:8080/ready
```

## Exercise

1. Tail API logs while creating an order in the UI
2. Port-forward and fetch `/metrics` — find `kubelab_orders_created_total`
3. Find a Failed event: `kubectl get events -n kubelab | grep Failed`
4. Open Cloud Logging and filter to kubelab namespace

## Next

→ [Module 10: CI/CD](10-cicd.md)
