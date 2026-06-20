# Module 02 — GKE Cluster

This module covers provisioning a GKE cluster with Terraform and connecting kubectl.

## Concepts

### GKE vs Self-Managed Kubernetes

GKE is Google's managed Kubernetes. Google handles:
- Control plane (API server, etcd, scheduler)
- Node OS patches and Kubernetes version upgrades
- Integration with GCP (Load Balancers, Persistent Disks, IAM)

You manage: workloads, manifests, application config.

### Standard vs Autopilot

| Mode | You manage | Best for |
|------|-----------|----------|
| **Standard** | Node pools, machine types | Learning, full control |
| **Autopilot** | Only pods | Production, hands-off ops |

This project uses **Standard** mode so you can see nodes, SSH, and node pools.

### Workload Identity

Pods need GCP permissions (e.g., read from Cloud Storage). **Workload Identity** maps a Kubernetes ServiceAccount to a GCP Service Account — no JSON key files in containers.

See `infra/terraform/main.tf` — we create a GCP SA and bind it to `kubelab/kubelab-api`.

## Provision the Cluster

```bash
export GCP_PROJECT_ID=your-project-id
make setup-gke
```

This runs Terraform which:
1. Enables required GCP APIs
2. Creates Artifact Registry repository `kubelab`
3. Creates GKE cluster with network policy enabled
4. Creates a 2-node pool (`e2-medium`)
5. Configures Workload Identity

### Review Terraform

Open `infra/terraform/main.tf` and understand each resource:

```hcl
# Key settings in our cluster:
network_policy { enabled = true }          # Required for NetworkPolicy
workload_identity_config { ... }           # Pod → GCP auth
release_channel { channel = "REGULAR" }    # Auto K8s upgrades
```

## Connect kubectl

After Terraform completes:

```bash
gcloud container clusters get-credentials kubelab-cluster \
  --region us-central1 \
  --project $GCP_PROJECT_ID

kubectl cluster-info
kubectl get nodes
```

You should see 2 nodes in `Ready` state.

## Explore the Cluster

```bash
# All system namespaces
kubectl get namespaces

# What's running on nodes (system pods)
kubectl get pods -A

# Node details (machine type, capacity)
kubectl describe nodes

# GKE-specific: check if metrics-server is running (needed for HPA)
kubectl get deployment metrics-server -n kube-system
```

## Exercise

1. Run `kubectl get nodes -o wide` — note internal/external IPs
2. Run `kubectl top nodes` — requires metrics-server (may show "metrics not available" briefly after cluster creation)
3. Open GCP Console → Kubernetes Engine → Clusters — find your cluster
4. Note the **Artifact Registry** URL from `terraform output`

## Teardown (save money)

```bash
cd infra/terraform
terraform destroy
```

## Next

→ [Module 03: Container Images](03-container-images.md)
