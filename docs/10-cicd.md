# Module 10 — CI/CD

Automating build, test, and deploy with GitHub Actions and GCP Workload Identity Federation.

## The Pipeline

```
git push (main) → GitHub Actions
                    ├── Test (compile Go, validate manifests)
                    ├── Build (Cloud Build → Artifact Registry, tag = git SHA)
                    └── Deploy (kubectl apply gke-dev overlay → GKE)
```

See `.github/workflows/ci.yaml`.

| Event | What runs |
|-------|-----------|
| Pull request | Test only |
| Push to `main` | Test → Build → Deploy |
| Manual (`workflow_dispatch`) | Test → Build (Deploy only on `main`) |

## One-Time GCP Setup (Cloud Shell)

Run once for your GCP project and repo `batmnnn/kube`:

```bash
cd ~/kube
chmod +x scripts/setup-github-cicd.sh

export GCP_PROJECT_ID=project-891d53eb-9710-4d69-954
export GKE_CLUSTER_NAME=kubelab-v2
export GKE_LOCATION=us-central1-a    # zone for zonal cluster (required for get-gke-credentials)
export GITHUB_REPO=batmnnn/kube

./scripts/setup-github-cicd.sh
```

This creates:

- Workload Identity pool + GitHub OIDC provider
- `github-ci@learning-deplo.iam.gserviceaccount.com` service account
- IAM: Artifact Registry writer, GKE developer, Cloud Build editor

**Prerequisites:** GKE cluster and Artifact Registry repo must already exist (`./scripts/cloud-shell-setup.sh`).

## GitHub Secrets

In https://github.com/batmnnn/kube → **Settings → Secrets and variables → Actions** → **New repository secret**:

| Secret | Example value |
|--------|---------------|
| `GCP_PROJECT_ID` | `project-891d53eb-9710-4d69-954` (project **ID**, not number) |
| `GKE_CLUSTER` | `kubelab-v2` |
| `GKE_LOCATION` | `us-central1-a` (zone for zonal cluster; use region e.g. `us-central1` for regional) |
| `WIF_PROVIDER` | `projects/396615866544/locations/global/workloadIdentityPools/github-pool/providers/github-provider` |
| `WIF_SERVICE_ACCOUNT` | `github-ci@project-891d53eb-9710-4d69-954.iam.gserviceaccount.com` |

The setup script prints the exact `WIF_PROVIDER` value for your project.

**All five secrets are required** for build/deploy. Without them, CI still runs the **Test** job but skips build/deploy with a warning (instead of failing on auth).

### Common error: `must specify exactly one of workload_identity_provider or credentials_json`

This means `WIF_PROVIDER` and/or `WIF_SERVICE_ACCOUNT` secrets are **missing or empty** in GitHub. Add all four secrets above, then re-run the workflow.

## What Each Job Does

### 1. Test (every PR and push)

- Compile Go API and worker
- Validate Kustomize overlays render without errors

### 2. Build & Push (not on PRs)

- Authenticate to GCP via Workload Identity Federation (no JSON keys)
- Build Docker images on the GitHub runner and push to Artifact Registry
- Tag images with git commit SHA: `us-central1-docker.pkg.dev/PROJECT/kubelab/{api,worker,frontend}:SHA`
- Also tags `:latest` on `main` branch

Cloud Shell builds still use `gcloud builds submit` via `cloudbuild.yaml`.

### 3. Deploy (main branch pushes only)

- `get-gke-credentials` for cluster + **location** (`GKE_LOCATION` secret — zone or region)
- `./scripts/deploy.sh gke-dev` with `IMAGE_TAG=$GITHUB_SHA`
- Waits for rollouts and runs an in-cluster smoke test

## Manifest Fixes Baked In

The repo manifests now include lessons from manual deploy:

- **NEG annotations** on `api` and `frontend` Services (required for GKE Ingress + ClusterIP)
- **Separate BackendConfigs** — API `:8080/health`, frontend `:80/`
- **Frontend nginx** — writable cache volumes + `NET_BIND_SERVICE`
- **gke-dev overlay** — 1 node / 1 replica each, tight CPU+memory requests, HPA capped at 1

## Manual Deploy (same as CI)

```bash
export GCP_PROJECT_ID=learning-deplo
export GCP_REGION=us-central1
export IMAGE_TAG=latest   # or a git SHA

gcloud builds submit . --config=cloudbuild.yaml \
  --substitutions="_REGION=${GCP_REGION},_TAG=${IMAGE_TAG}"

./scripts/deploy.sh gke-dev
```

Or:

```bash
make push TAG=abc123
make deploy TAG=abc123 OVERLAY=gke-dev
```

## Trigger CI

```bash
git add .
git commit -m "feat: add CI/CD pipeline"
git push origin main
```

Watch runs at: https://github.com/batmnnn/kube/actions

## Troubleshooting CI

| Failure | Fix |
|---------|-----|
| `Permission denied` on WIF auth | Re-run `./scripts/setup-github-cicd.sh`; verify GitHub secrets |
| `cannot patch resource "roles"` in deploy | Re-run `./scripts/setup-github-cicd.sh` (grants `container.admin` to CI SA) |
| `Invalid bucket name ..._cloudbuild` | `GCP_PROJECT_ID` secret must be project ID only (e.g. `learning-deplo`); re-run setup script to create staging bucket |
| `forbidden from accessing the bucket` | CI builds with Docker (no GCS bucket). For Cloud Shell, re-run `./scripts/setup-github-cicd.sh` |
| Cloud Build push fails | Ensure Cloud Build SA has `artifactregistry.writer` (cloud-shell-setup.sh) |
| Deploy `ImagePullBackOff` | Check `IMAGE_TAG` in overlay matches built SHA |
| Rollout timeout | gke-dev uses 10m CPU requests + maxUnavailable=1; check Pending pods with `kubectl get pods -n kubelab` |
| `get-gke-credentials` not found | Set `GKE_LOCATION` to the cluster **zone** (`us-central1-a`) not just the region |
| Ingress no IP | NEG + BackendConfig are in manifests now; wait 5–15 min after first deploy |

## GitOps Alternative

For production teams, consider **Argo CD** or **Flux** — they watch git and sync cluster state automatically. Our push-based CI is simpler for learning.

## Next

→ [Module 11: Production Checklist](11-production-checklist.md)
