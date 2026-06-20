# Module 04 — Kubernetes Basics

The core objects that run your application.

## The Hierarchy

```
Cluster
 └── Namespace (kubelab)
      └── Deployment (api)
           └── ReplicaSet (managed automatically)
                └── Pod (api-xxxxx-yyyyy)
                     └── Container (api)
```

## Pod

The smallest deployable unit. One or more containers sharing network/storage.

```bash
kubectl get pods -n kubelab
kubectl describe pod <name> -n kubelab
kubectl logs <pod-name> -n kubelab -f
```

**Key insight:** You almost never create Pods directly. Deployments manage them.

## Deployment

Declares desired state: "run 2 replicas of this container image."

```bash
kubectl apply -f k8s/base/api/deployment.yaml
kubectl rollout status deployment/api -n kubelab
kubectl rollout history deployment/api -n kubelab
```

### Rolling Updates

When you change the image or config, Deployment performs a rolling update:

```bash
# Update image
kubectl set image deployment/api api=us-central1-docker.pkg.dev/PROJECT/kubelab/api:v2 -n kubelab

# Watch rollout
kubectl rollout status deployment/api -n kubelab

# Rollback if broken
kubectl rollout undo deployment/api -n kubelab
```

Our Deployment sets `maxUnavailable: 0` for zero-downtime updates.

## Service

Pods are ephemeral — they get new IPs when restarted. **Services** provide stable DNS and load balancing.

```bash
kubectl get svc -n kubelab
```

| Service | Type | DNS Name | Purpose |
|---------|------|----------|---------|
| api | ClusterIP | `api.kubelab.svc.cluster.local` | Internal API access |
| frontend | ClusterIP | `frontend.kubelab.svc` | Internal frontend |
| postgres | Headless | `postgres-0.postgres.kubelab.svc` | StatefulSet DNS |

Test from inside the cluster:

```bash
kubectl run curl --rm -it --image=curlimages/curl -n kubelab -- \
  curl -s http://api:8080/health
```

## Probes

Kubernetes uses HTTP probes to know pod health:

| Probe | Question | Failure action |
|-------|----------|----------------|
| **startup** | Has the app finished booting? | Keep waiting |
| **liveness** | Is the process alive? | Restart pod |
| **readiness** | Can it serve traffic? | Remove from Service |

Our API (`k8s/base/api/deployment.yaml`):
- `/health` — liveness (process up)
- `/ready` — readiness (DB + Redis connected)

```bash
# See probe failures in events
kubectl describe pod -l app.kubernetes.io/name=api -n kubelab | grep -A5 Events
```

## Init Containers

Run before main containers start. We use them to wait for postgres/redis:

```yaml
initContainers:
  - name: wait-for-postgres
    image: busybox:1.36
    command: ['sh', '-c', 'until nc -z postgres 5432; do sleep 2; done']
```

## Deploy Everything

```bash
make deploy OVERLAY=gke-dev
# or
kubectl apply -k k8s/overlays/gke-dev  # after substituting PROJECT_ID
```

## Exercise

1. Scale API manually: `kubectl scale deployment/api --replicas=3 -n kubelab`
2. Watch pods: `kubectl get pods -n kubelab -w`
3. Delete a pod: `kubectl delete pod -l app.kubernetes.io/name=api -n kubelab` — watch Deployment recreate it
4. Port-forward API: `kubectl port-forward svc/api -n kubelab 8080:8080` → curl localhost:8080/api/orders
5. Read the full Deployment manifest and identify: replicas, probes, resources, env vars

## Next

→ [Module 05: Networking](05-networking.md)
