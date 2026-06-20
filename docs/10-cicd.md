# Module 10 — CI/CD

Automating build, test, and deploy with GitHub Actions and GCP Workload Identity Federation.

## The Pipeline

```
git push → GitHub Actions → Build images → Push to Artifact Registry → Deploy to GKE
```

See `.github/workflows/ci.yaml`.

## Stages

### 1. Test (every PR and push)

- Compile Go API and worker
- Validate Kustomize manifests render without errors

### 2. Build & Push (main branch only)

- Authenticate to GCP via Workload Identity Federation (no JSON keys!)
- Build and push all 3 images tagged with git SHA
- Deploy to GKE using `scripts/deploy.sh`

## Workload Identity Federation Setup

This lets GitHub Actions authenticate to GCP without storing service account keys.

### 1. Create WIF Pool and Provider

```bash
gcloud iam workload-identity-pools create github-pool \
  --location=global \
  --display-name="GitHub Actions"

gcloud iam workload-identity-pools providers create-oidc github-provider \
  --location=global \
  --workload-identity-pool=github-pool \
  --issuer-uri=https://token.actions.githubusercontent.com \
  --attribute-mapping="google.subject=assertion.sub,attribute.repository=assertion.repository" \
  --attribute-condition="assertion.repository=='YOUR_ORG/kube'"
```

### 2. Create CI Service Account

```bash
gcloud iam service-accounts create github-ci \
  --display-name="GitHub CI"

gcloud projects add-iam-policy-binding $GCP_PROJECT_ID \
  --member="serviceAccount:github-ci@$GCP_PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/artifactregistry.writer"

gcloud projects add-iam-policy-binding $GCP_PROJECT_ID \
  --member="serviceAccount:github-ci@$GCP_PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/container.developer"
```

### 3. GitHub Secrets

Add to your GitHub repo:

| Secret | Value |
|--------|-------|
| `GCP_PROJECT_ID` | your-project-id |
| `GKE_CLUSTER` | kubelab-cluster |
| `WIF_PROVIDER` | projects/PROJECT_NUMBER/locations/global/workloadIdentityPools/github-pool/providers/github-provider |
| `WIF_SERVICE_ACCOUNT` | github-ci@PROJECT.iam.gserviceaccount.com |

## Kustomize Overlays in CI

Different environments use different overlays:

```bash
# Dev — latest tags, 2 replicas
./scripts/deploy.sh gke-dev

# Prod — semver tags, 3+ replicas, stricter config
./scripts/deploy.sh gke-prod
```

## Manual Deploy (without CI)

```bash
make push TAG=v1.0.0
IMAGE_TAG=v1.0.0 make deploy OVERLAY=gke-prod
```

## GitOps Alternative

For production teams, consider **Argo CD** or **Flux** — they watch git and sync cluster state automatically. Our push-based CI is simpler for learning.

## Exercise

1. Read `.github/workflows/ci.yaml` line by line
2. Run the test job locally: `cd app/api && go build .`
3. Validate manifests: `kubectl kustomize k8s/base | head -50`
4. Set up WIF (optional) and push to GitHub to trigger CI

## Next

→ [Module 11: Production Checklist](11-production-checklist.md)
