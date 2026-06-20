#!/usr/bin/env bash
# Tear down GKE cluster and Artifact Registry from Cloud Shell.
set -euo pipefail

PROJECT_ID="${GCP_PROJECT_ID:-$(gcloud config get-value project 2>/dev/null)}"
REGION="${GCP_REGION:-us-central1}"
CLUSTER_NAME="${GKE_CLUSTER_NAME:-kubelab-cluster}"

echo "WARNING: This deletes the GKE cluster and stops billing for nodes."
read -rp "Delete cluster '${CLUSTER_NAME}'? [y/N] " confirm
if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
  echo "Cancelled."
  exit 0
fi

gcloud container clusters delete "$CLUSTER_NAME" \
  --region="$REGION" \
  --project="$PROJECT_ID" \
  --quiet

echo ""
read -rp "Also delete Artifact Registry repo 'kubelab'? [y/N] " confirm_repo
if [[ "$confirm_repo" =~ ^[Yy]$ ]]; then
  gcloud artifacts repositories delete kubelab \
    --location="$REGION" \
    --project="$PROJECT_ID" \
    --quiet
fi

echo "Done. Run 'kubectl get nodes' — should fail (no cluster)."
