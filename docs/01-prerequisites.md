# Module 01 — Prerequisites

Before deploying to GKE, install and configure these tools.

## Required Tools

| Tool | Purpose | Install |
|------|---------|---------|
| **gcloud CLI** | Manage GCP resources | [Install guide](https://cloud.google.com/sdk/docs/install) |
| **kubectl** | Talk to Kubernetes | `gcloud components install kubectl` |
| **Docker** | Build container images | [Docker Desktop](https://www.docker.com/products/docker-desktop/) |
| **Terraform** | Provision GKE cluster | [terraform.io](https://developer.hashicorp.com/terraform/install) |
| **kustomize** | Template K8s manifests | Built into kubectl 1.14+ |

Optional but recommended:
- **kind** — local Kubernetes cluster (`brew install kind`)
- **k9s** — terminal UI for kubectl (`brew install k9s`)

## GCP Setup

### 1. Create a GCP Project

```bash
gcloud projects create kubelab-learn --name="KubeLab Learning"
gcloud config set project kubelab-learn
```

Or use an existing project. Note your **Project ID** (not display name).

### 2. Enable Billing

GKE requires a billing account. Link it in the [GCP Console](https://console.cloud.google.com/billing).

### 3. Authenticate

```bash
gcloud auth login
gcloud auth application-default login
```

### 4. Set Defaults

```bash
export GCP_PROJECT_ID=your-project-id
export GCP_REGION=us-central1

gcloud config set project $GCP_PROJECT_ID
gcloud config set compute/region $GCP_REGION
```

## Verify Installation

Run this checklist:

```bash
gcloud --version          # Google Cloud SDK
kubectl version --client  # Kubernetes CLI
docker --version          # Container runtime
terraform --version       # Infrastructure as code
kubectl kustomize --help  # Kustomize (built-in)
```

## Exercise

1. Run `gcloud config list` and confirm project and region
2. Run `docker run hello-world` to verify Docker works
3. Clone/explore this repo structure with `tree -L 2` or `ls -R`

## Next

→ [Module 02: GKE Cluster](02-gke-cluster.md)
