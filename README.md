# KubeLab — Learn Full GKE Deployment

A hands-on order-processing application designed to teach **every major concept** in deploying to **Google Kubernetes Engine (GKE)**. Not a toy hello-world — a realistic multi-service app with annotated manifests, Terraform infrastructure, and a structured curriculum.

## What You'll Learn

| Module | Topics |
|--------|--------|
| [01 Prerequisites](docs/01-prerequisites.md) | gcloud, kubectl, Docker, billing |
| [02 GKE Cluster](docs/02-gke-cluster.md) | Terraform, node pools, Workload Identity |
| [03 Container Images](docs/03-container-images.md) | Dockerfiles, multi-stage builds, Artifact Registry |
| [04 Kubernetes Basics](docs/04-kubernetes-basics.md) | Pods, Deployments, Services, probes |
| [05 Networking](docs/05-networking.md) | ClusterIP, Ingress, GCE Load Balancer |
| [06 Storage](docs/06-storage.md) | PVCs, StatefulSets, GKE storage classes |
| [07 Secrets & Config](docs/07-secrets-config.md) | ConfigMaps, Secrets, external secret management |
| [08 Scaling & Resilience](docs/08-scaling-resilience.md) | HPA, PDB, rolling updates |
| [09 Observability](docs/09-observability.md) | Logs, metrics, health checks |
| [10 CI/CD](docs/10-cicd.md) | GitHub Actions, Workload Identity Federation |
| [11 Production Checklist](docs/11-production-checklist.md) | HTTPS, backups, cost, security |

## Architecture

```
                    ┌─────────────────────────────────────┐
                    │     GKE Ingress (GCE LB)            │
                    │     http://EXTERNAL_IP              │
                    └──────────┬──────────────┬───────────┘
                               │              │
                    ┌──────────▼──┐    ┌──────▼──────┐
                    │  Frontend   │    │     API     │
                    │  (nginx)    │───▶│   (Go)      │
                    │  Deployment │    │  Deployment │
                    └─────────────┘    └──┬───────┬────┘
                                          │       │
                              ┌───────────▼┐   ┌──▼────────┐
                              │ PostgreSQL │   │   Redis   │
                              │ StatefulSet│   │ Deployment│
                              │    + PVC   │   │  (queue)  │
                              └────────────┘   └─────┬─────┘
                                                     │
                                              ┌──────▼──────┐
                                              │   Worker    │
                                              │ Deployment  │
                                              └─────────────┘
```

## Kubernetes Resources Included

Every resource type below is deployed and documented:

- **Namespace** with Pod Security Standards
- **Deployment** — API, Frontend, Worker, Redis
- **StatefulSet** — PostgreSQL with persistent volume
- **Service** — ClusterIP (internal) + headless (StatefulSet)
- **Ingress** — GKE HTTP(S) Load Balancer
- **BackendConfig** — GKE-specific LB tuning
- **ConfigMap** — non-sensitive configuration
- **Secret** — database credentials
- **ServiceAccount + RBAC** — least-privilege access
- **HorizontalPodAutoscaler** — CPU-based autoscaling
- **PodDisruptionBudget** — safe cluster upgrades
- **NetworkPolicy** — pod-to-pod firewall rules
- **Job** — one-time data seeding
- **CronJob** — scheduled cleanup
- **Init Containers** — wait for dependencies

## Quick Start

### Option A: Local (Docker Compose) — no cluster needed

```bash
make dev
# Open http://localhost:3000
```

### Option B: Local Kubernetes (kind)

```bash
make local-k8s
kubectl port-forward svc/frontend -n kubelab 3000:80
# Open http://localhost:3000
```

### Option C: Full GKE Deployment

```bash
# 1. Set your GCP project
export GCP_PROJECT_ID=your-project-id
gcloud config set project $GCP_PROJECT_ID

# 2. Create cluster + Artifact Registry
make setup-gke

# 3. Build and push images
make push

# 4. Deploy to GKE
make deploy

# 5. Get the external IP (may take 5-10 min)
kubectl get ingress kubelab-ingress -n kubelab -w
```

## Project Structure

```
kube/
├── app/
│   ├── api/           # Go REST API (orders CRUD, health, metrics)
│   ├── worker/        # Background queue processor
│   └── frontend/      # Static UI + nginx reverse proxy
├── k8s/
│   ├── base/          # All Kubernetes manifests
│   └── overlays/      # Kustomize env-specific configs
│       ├── local/
│       ├── gke-dev/
│       └── gke-prod/
├── infra/terraform/   # GKE cluster + Artifact Registry
├── scripts/           # Setup, build, deploy automation
├── docs/              # Step-by-step learning modules
├── docker-compose.yml # Local development
└── Makefile           # Common commands
```

## Learning Path

Follow the docs in order. Each module includes **exercises** — hands-on commands to run and concepts to verify.

1. Start with **Docker Compose** (`make dev`) to understand the app
2. Read **docs/04-kubernetes-basics.md** while exploring manifests in `k8s/base/`
3. Deploy to **kind** (`make local-k8s`) to practice kubectl without cloud costs
4. Provision **GKE** with Terraform and deploy for real
5. Work through scaling, networking, and production hardening modules

## Useful Commands

```bash
# Watch pods come up
kubectl get pods -n kubelab -w

# Describe a failing pod
kubectl describe pod -l app.kubernetes.io/name=api -n kubelab

# Shell into postgres
kubectl exec -it postgres-0 -n kubelab -- psql -U kubelab

# Trigger HPA test (requires load generator)
kubectl run -it loadgen --rm --image=busybox -n kubelab -- sh -c "while true; do wget -qO- http://api:8080/api/orders; done"

# View HPA status
kubectl get hpa -n kubelab

# Check NetworkPolicies
kubectl get networkpolicy -n kubelab

# View CronJob history
kubectl get cronjobs,jobs -n kubelab
```

## Cost Warning

A GKE cluster with 2 `e2-medium` nodes costs roughly **$50-70/month**. Delete when done:

```bash
cd infra/terraform && terraform destroy
```

Or use **GKE Autopilot** (pay-per-pod) for lower idle costs — see docs/02-gke-cluster.md.

## License

MIT — use freely for learning.
