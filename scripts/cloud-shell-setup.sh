#!/usr/bin/env bash
# One-time GCP setup from Google Cloud Shell (no Terraform, no local Docker).
# Creates Artifact Registry + GKE cluster and configures kubectl.
set -euo pipefail

PROJECT_ID="${GCP_PROJECT_ID:-$(gcloud config get-value project 2>/dev/null)}"
REGION="${GCP_REGION:-us-central1}"
CLUSTER_NAME="${GKE_CLUSTER_NAME:-kubelab-cluster}"
NODE_COUNT="${GKE_NODE_COUNT:-2}"

if [[ -z "$PROJECT_ID" || "$PROJECT_ID" == "(unset)" ]]; then
  echo "Error: set a project first:"
  echo "  gcloud config set project YOUR_PROJECT_ID"
  exit 1
fi

echo "==> KubeLab Cloud Shell Setup"
echo "    Project:  $PROJECT_ID"
echo "    Region:   $REGION"
echo "    Cluster:  $CLUSTER_NAME"
echo ""

echo "==> Enabling APIs (takes ~1 min)"
gcloud services enable \
  container.googleapis.com \
  artifactregistry.googleapis.com \
  compute.googleapis.com \
  cloudbuild.googleapis.com \
  --project="$PROJECT_ID"

echo ""
echo "==> Creating Artifact Registry repo 'kubelab'"
if gcloud artifacts repositories describe kubelab \
  --location="$REGION" --project="$PROJECT_ID" &>/dev/null; then
  echo "    Already exists — skipping"
else
  gcloud artifacts repositories create kubelab \
    --repository-format=docker \
    --location="$REGION" \
    --description="KubeLab container images" \
    --project="$PROJECT_ID"
fi

echo ""
echo "==> Granting Cloud Build permission to push images"
PROJECT_NUMBER=$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)')
CB_SA="${PROJECT_NUMBER}@cloudbuild.gserviceaccount.com"
gcloud artifacts repositories add-iam-policy-binding kubelab \
  --location="$REGION" \
  --member="serviceAccount:${CB_SA}" \
  --role="roles/artifactregistry.writer" \
  --project="$PROJECT_ID" --quiet

echo ""
echo "==> Creating GKE cluster (takes ~5-10 min)"
if gcloud container clusters describe "$CLUSTER_NAME" \
  --region="$REGION" --project="$PROJECT_ID" &>/dev/null; then
  echo "    Cluster already exists — skipping"
else
  gcloud container clusters create "$CLUSTER_NAME" \
    --region="$REGION" \
    --num-nodes="$NODE_COUNT" \
    --machine-type=e2-medium \
    --enable-network-policy \
    --workload-pool="${PROJECT_ID}.svc.id.goog" \
    --release-channel=regular \
    --logging=SYSTEM,WORKLOAD \
    --monitoring=SYSTEM \
    --project="$PROJECT_ID"
fi

echo ""
echo "==> Configuring kubectl"
gcloud container clusters get-credentials "$CLUSTER_NAME" \
  --region="$REGION" \
  --project="$PROJECT_ID"

echo ""
echo "==> Verifying cluster"
kubectl get nodes

echo ""
echo "Setup complete!"
echo ""
echo "Next steps:"
echo "  ./scripts/cloud-shell-build.sh"
echo "  ./scripts/deploy.sh gke-dev"
