#!/usr/bin/env bash
# One-time GCP setup from Google Cloud Shell (no Terraform, no local Docker).
# Creates Artifact Registry + GKE cluster and configures kubectl.
set -euo pipefail

PROJECT_ID="${GCP_PROJECT_ID:-$(gcloud config get-value project 2>/dev/null)}"
REGION="${GCP_REGION:-us-central1}"
# Set GKE_ZONE (e.g. us-central1-a) for a true single-node zonal cluster — saves CPU quota vs regional.
GKE_ZONE="${GKE_ZONE:-}"
CLUSTER_NAME="${GKE_CLUSTER_NAME:-kubelab-cluster}"
NODE_COUNT="${GKE_NODE_COUNT:-1}"
MACHINE_TYPE="${GKE_MACHINE_TYPE:-e2-medium}"
# GKE default boot disk is 100GB per node — burns SSD quota fast on trial projects.
DISK_SIZE_GB="${GKE_DISK_SIZE:-30}"

if [[ -n "$GKE_ZONE" ]]; then
  LOCATION="$GKE_ZONE"
  LOCATION_ARGS=(--zone="$GKE_ZONE")
else
  LOCATION="$REGION"
  LOCATION_ARGS=(--region="$REGION")
fi

if [[ -z "$PROJECT_ID" || "$PROJECT_ID" == "(unset)" ]]; then
  echo "Error: set a project first:"
  echo "  gcloud config set project YOUR_PROJECT_ID"
  exit 1
fi

echo "==> KubeLab Cloud Shell Setup"
echo "    Project:  $PROJECT_ID"
echo "    Location: $LOCATION ($([ -n "$GKE_ZONE" ] && echo zonal || echo regional))"
echo "    Cluster:  $CLUSTER_NAME"
echo "    Nodes:    $NODE_COUNT × $MACHINE_TYPE (${DISK_SIZE_GB}GB disk each)"
if [[ -z "$GKE_ZONE" && "$NODE_COUNT" == "1" ]]; then
  echo "    Tip: regional + num-nodes=1 creates 1 node per zone (~3 nodes). Set GKE_ZONE=us-central1-a for a single node."
fi
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
echo "==> Creating Cloud Build staging bucket (if missing)"
CB_BUCKET="${PROJECT_ID}_cloudbuild"
if gcloud storage buckets describe "gs://${CB_BUCKET}" --project="$PROJECT_ID" &>/dev/null; then
  echo "    Already exists — skipping"
else
  gcloud storage buckets create "gs://${CB_BUCKET}" \
    --location="$REGION" \
    --uniform-bucket-level-access \
    --project="$PROJECT_ID"
fi

echo ""
echo "==> Granting Cloud Build + GKE node permissions"
PROJECT_NUMBER=$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)')
CB_SA="${PROJECT_NUMBER}@cloudbuild.gserviceaccount.com"
COMPUTE_SA="${PROJECT_NUMBER}-compute@developer.gserviceaccount.com"
gcloud artifacts repositories add-iam-policy-binding kubelab \
  --location="$REGION" \
  --member="serviceAccount:${CB_SA}" \
  --role="roles/artifactregistry.writer" \
  --project="$PROJECT_ID" --quiet
# Cloud Build reads uploaded source from the staging bucket; new projects often deny this by default.
gcloud storage buckets add-iam-policy-binding "gs://${CB_BUCKET}" \
  --member="serviceAccount:${CB_SA}" \
  --role="roles/storage.objectAdmin" \
  --project="$PROJECT_ID" --quiet
gcloud storage buckets add-iam-policy-binding "gs://${CB_BUCKET}" \
  --member="serviceAccount:${COMPUTE_SA}" \
  --role="roles/storage.objectViewer" \
  --project="$PROJECT_ID" --quiet
# GKE nodes pull images; Cloud Build in new projects often runs as the compute default SA.
gcloud artifacts repositories add-iam-policy-binding kubelab \
  --location="$REGION" \
  --member="serviceAccount:${COMPUTE_SA}" \
  --role="roles/artifactregistry.reader" \
  --project="$PROJECT_ID" --quiet
gcloud artifacts repositories add-iam-policy-binding kubelab \
  --location="$REGION" \
  --member="serviceAccount:${COMPUTE_SA}" \
  --role="roles/artifactregistry.writer" \
  --project="$PROJECT_ID" --quiet
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:${COMPUTE_SA}" \
  --role="roles/logging.logWriter" \
  --quiet

echo ""
echo "==> Creating GKE cluster (takes ~5-10 min)"
if gcloud container clusters describe "$CLUSTER_NAME" \
  "${LOCATION_ARGS[@]}" --project="$PROJECT_ID" &>/dev/null; then
  echo "    Cluster already exists — skipping"
else
  gcloud container clusters create "$CLUSTER_NAME" \
    "${LOCATION_ARGS[@]}" \
    --num-nodes="$NODE_COUNT" \
    --machine-type="$MACHINE_TYPE" \
    --disk-size="$DISK_SIZE_GB" \
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
  "${LOCATION_ARGS[@]}" \
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
