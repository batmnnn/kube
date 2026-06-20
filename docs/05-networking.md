# Module 05 — Networking

How traffic flows from the internet to your pods on GKE.

## Kubernetes Networking Model

- Every pod gets a unique IP (flat network)
- Pods can talk to each other without NAT
- Services provide stable virtual IPs via kube-proxy/CNI

## Service Types

| Type | Reachable from | GKE behavior |
|------|---------------|--------------|
| **ClusterIP** | Inside cluster only | Default — our API, frontend, redis |
| **NodePort** | Node IP + port | Rarely used directly |
| **LoadBalancer** | External IP | Creates GCP TCP/UDP LB |
| **Ingress** | HTTP/HTTPS routes | Creates GCP HTTP(S) LB — **we use this** |

## Our Ingress Setup

`k8s/base/ingress/ingress.yaml` routes:

```
/          → frontend:80
/api/*     → api:8080
/health    → api:8080
/ready     → api:8080
```

When you apply Ingress on GKE:
1. GKE Ingress Controller reads the Ingress resource
2. Creates a **Google Cloud HTTP(S) Load Balancer**
3. Configures backend services pointing to NodePort/NEG backends
4. Assigns an external IP

```bash
kubectl get ingress kubelab-ingress -n kubelab
# Wait for ADDRESS column to populate (5-10 minutes)
```

## BackendConfig (GKE-Specific)

`k8s/base/ingress/backendconfig.yaml` tunes the GCP Load Balancer:
- Health check path/interval
- Connection draining during rollouts
- Timeout settings

## DNS Inside the Cluster

Pods resolve Services automatically:

```
api.kubelab.svc.cluster.local     → ClusterIP of api Service
postgres-0.postgres.kubelab.svc   → Pod IP of postgres-0 (headless)
```

Short names work within the same namespace: `http://api:8080`

## NetworkPolicies

With `network_policy { enabled = true }` in Terraform, we deploy firewall rules:

```bash
kubectl get networkpolicy -n kubelab
```

- **default-deny-ingress** — block all incoming traffic by default
- **allow-frontend-to-api** — only frontend/worker can reach API
- **allow-app-to-postgres** — only api/worker can reach DB

Test isolation:

```bash
# This should fail (redis blocks unknown pods)
kubectl run test --rm -it --image=busybox -n kubelab -- nc -zv postgres 5432
```

## HTTPS (Production)

For production, add a ManagedCertificate:

```yaml
apiVersion: networking.gke.io/v1
kind: ManagedCertificate
metadata:
  name: kubelab-cert
spec:
  domains:
    - kubelab.yourdomain.com
```

And update Ingress annotations — see Module 11.

## Exercise

1. Get Ingress IP and curl the app: `curl http://INGRESS_IP/api/orders`
2. Trace a request: Ingress → Service → Pod (draw the path)
3. Run `kubectl get endpoints -n kubelab` — see which pod IPs back each Service
4. Read NetworkPolicy manifests and explain why worker can reach postgres but frontend cannot

## Next

→ [Module 06: Storage](06-storage.md)
