#!/usr/bin/env bash
# One-time setup: GitHub Actions → GCP via Workload Identity Federation.
# Run from Cloud Shell (or locally with gcloud auth).
#
# Usage:
#   export GCP_PROJECT_ID=learning-deplo
#   export GITHUB_REPO=batmnnn/kube    # owner/repo
#   ./scripts/setup-github-cicd.sh
#
set -euo pipefail

PROJECT_ID="${GCP_PROJECT_ID:-$(gcloud config get-value project 2>/dev/null)}"
REGION="${GCP_REGION:-us-central1}"
GITHUB_REPO="${GITHUB_REPO:-batmnnn/kube}"
POOL_ID="${WIF_POOL_ID:-github-pool}"
PROVIDER_ID="${WIF_PROVIDER_ID:-github-provider}"
SA_NAME="${CI_SA_NAME:-github-ci}"
SA_EMAIL="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

if [[ -z "$PROJECT_ID" || "$PROJECT_ID" == "(unset)" ]]; then
  echo "Error: set GCP_PROJECT_ID or gcloud config project"
  exit 1
fi

PROJECT_NUMBER=$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)')
WIF_PROVIDER="projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${POOL_ID}/providers/${PROVIDER_ID}"

echo "==> KubeLab GitHub CI/CD Setup"
echo "    Project:     $PROJECT_ID"
echo "    GitHub repo: $GITHUB_REPO"
echo "    CI SA:       $SA_EMAIL"
echo ""

echo "==> Enabling APIs"
gcloud services enable \
  iam.googleapis.com \
  iamcredentials.googleapis.com \
  cloudresourcemanager.googleapis.com \
  sts.googleapis.com \
  storage.googleapis.com \
  artifactregistry.googleapis.com \
  container.googleapis.com \
  cloudbuild.googleapis.com \
  --project="$PROJECT_ID"

echo ""
echo "==> Creating Workload Identity pool (if missing)"
if gcloud iam workload-identity-pools describe "$POOL_ID" \
  --location=global --project="$PROJECT_ID" &>/dev/null; then
  echo "    Pool exists — skipping"
else
  gcloud iam workload-identity-pools create "$POOL_ID" \
    --location=global \
    --display-name="GitHub Actions" \
    --project="$PROJECT_ID"
fi

echo ""
echo "==> Creating OIDC provider for GitHub (if missing)"
if gcloud iam workload-identity-pools providers describe "$PROVIDER_ID" \
  --location=global --workload-identity-pool="$POOL_ID" --project="$PROJECT_ID" &>/dev/null; then
  echo "    Provider exists — skipping"
else
  gcloud iam workload-identity-pools providers create-oidc "$PROVIDER_ID" \
    --location=global \
    --workload-identity-pool="$POOL_ID" \
    --display-name="GitHub provider" \
    --issuer-uri="https://token.actions.githubusercontent.com" \
    --attribute-mapping="google.subject=assertion.sub,attribute.actor=assertion.actor,attribute.repository=assertion.repository,attribute.repository_owner=assertion.repository_owner" \
    --attribute-condition="assertion.repository=='${GITHUB_REPO}'" \
    --project="$PROJECT_ID"
fi

echo ""
echo "==> Creating CI service account (if missing)"
if gcloud iam service-accounts describe "$SA_EMAIL" --project="$PROJECT_ID" &>/dev/null; then
  echo "    Service account exists — skipping"
else
  gcloud iam service-accounts create "$SA_NAME" \
    --display-name="GitHub Actions CI" \
    --project="$PROJECT_ID"
fi

echo ""
echo "==> Creating Cloud Build staging bucket (if missing)"
CB_BUCKET="${PROJECT_ID}_cloudbuild"
if gcloud storage buckets describe "gs://${CB_BUCKET}" --project="$PROJECT_ID" &>/dev/null; then
  echo "    Bucket exists — skipping"
else
  gcloud storage buckets create "gs://${CB_BUCKET}" \
    --location="$REGION" \
    --uniform-bucket-level-access \
    --project="$PROJECT_ID"
  echo "    Created gs://${CB_BUCKET}"
fi

echo ""
echo "==> Granting CI service account bucket access"
gcloud storage buckets add-iam-policy-binding "gs://${CB_BUCKET}" \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/storage.objectAdmin" \
  --project="$PROJECT_ID" \
  --quiet

echo ""
echo "==> Granting CI service account permissions"
for role in \
  roles/artifactregistry.writer \
  roles/container.developer \
  roles/cloudbuild.builds.editor \
  roles/storage.admin \
  roles/serviceusage.serviceUsageConsumer; do
  gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:${SA_EMAIL}" \
    --role="$role" \
    --quiet >/dev/null
  echo "    $role"
done

echo ""
echo "==> Allowing GitHub repo to impersonate CI service account"
gcloud iam service-accounts add-iam-policy-binding "$SA_EMAIL" \
  --project="$PROJECT_ID" \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${POOL_ID}/attribute.repository/${GITHUB_REPO}" \
  --quiet

echo ""
echo "==> Setup complete!"
echo ""
echo "Add these GitHub repository secrets (Settings → Secrets → Actions):"
echo ""
echo "  GCP_PROJECT_ID         = ${PROJECT_ID}"
echo "  GKE_CLUSTER            = kubelab-cluster"
echo "  GCP_REGION             = ${REGION}   (optional — workflow defaults to us-central1)"
echo "  WIF_PROVIDER           = ${WIF_PROVIDER}"
echo "  WIF_SERVICE_ACCOUNT    = ${SA_EMAIL}"
echo ""
echo "Then push to main on https://github.com/${GITHUB_REPO} to trigger CI/CD."
