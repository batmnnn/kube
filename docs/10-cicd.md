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

Run once for project `learning-deplo` and repo `batmnnn/kube`:

```bash
cd ~/kube
chmod +x scripts/setup-github-cicd.sh

export GCP_PROJECT_ID=learning-deplo
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
| `GCP_PROJECT_ID` | `learning-deplo` (project **ID**, not number — no spaces/newlines) |
| `GKE_CLUSTER` | `kubelab-cluster` |
| `WIF_PROVIDER` | `projects/663804652181/locations/global/workloadIdentityPools/github-pool/providers/github-provider` |
| `WIF_SERVICE_ACCOUNT` | `github-ci@learning-deplo.iam.gserviceaccount.com` |

The setup script prints the exact `WIF_PROVIDER` value for your project.

**All four secrets are required** for build/deploy. Without them, CI still runs the **Test** job but skips build/deploy with a warning (instead of failing on auth).

### Common error: `must specify exactly one of workload_identity_provider or credentials_json`

This means `WIF_PROVIDER` and/or `WIF_SERVICE_ACCOUNT` secrets are **missing or empty** in GitHub. Add all four secrets above, then re-run the workflow.

## What Each Job Does

### 1. Test (every PR and push)

- Compile Go API and worker
- Validate Kustomize overlays render without errors

### 2. Build & Push (not on PRs)

- Authenticate to GCP via Workload Identity Federation (no JSON keys)
- Run `gcloud builds submit` using `cloudbuild.yaml`
- Tag images with git commit SHA: `us-central1-docker.pkg.dev/PROJECT/kubelab/{api,worker,frontend}:SHA`
- Also tags `:latest` on `main` branch

### 3. Deploy (main branch pushes only)

- `get-gke-credentials` for `kubelab-cluster`
- `./scripts/deploy.sh gke-dev` with `IMAGE_TAG=$GITHUB_SHA`
- Waits for rollouts and runs an in-cluster smoke test

## Manifest Fixes Baked In

The repo manifests now include lessons from manual deploy:

- **NEG annotations** on `api` and `frontend` Services (required for GKE Ingress + ClusterIP)
- **Separate BackendConfigs** — API `:8080/health`, frontend `:80/`
- **Frontend nginx** — writable cache volumes + `NET_BIND_SERVICE`
- **gke-dev overlay** — 1 replica each, HPA min=1 (fits small trial clusters)

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
| `Invalid bucket name ..._cloudbuild` | `GCP_PROJECT_ID` secret must be project ID only (e.g. `learning-deplo`); re-run setup script to create staging bucket |
| Cloud Build push fails | Ensure Cloud Build SA has `artifactregistry.writer` (cloud-shell-setup.sh) |
| Deploy `ImagePullBackOff` | Check `IMAGE_TAG` in overlay matches built SHA |
| Rollout timeout | Cluster CPU full — gke-dev uses 1 replica; delete Pending pods |
| Ingress no IP | NEG + BackendConfig are in manifests now; wait 5–15 min after first deploy |

## GitOps Alternative

For production teams, consider **Argo CD** or **Flux** — they watch git and sync cluster state automatically. Our push-based CI is simpler for learning.

## Next

→ [Module 11: Production Checklist](11-production-checklist.md)
