#!/usr/bin/env bash
# Build and push images using Cloud Build (no local Docker required).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

PROJECT_ID="${GCP_PROJECT_ID:-$(gcloud config get-value project 2>/dev/null)}"
REGION="${GCP_REGION:-us-central1}"
TAG="${IMAGE_TAG:-latest}"

if [[ -z "$PROJECT_ID" || "$PROJECT_ID" == "(unset)" ]]; then
  echo "Error: gcloud project not set"
  exit 1
fi

echo "==> Building images with Cloud Build"
echo "    Project: $PROJECT_ID"
echo "    Region:  $REGION"
echo "    Tag:     $TAG"
echo ""

cd "$ROOT_DIR"
gcloud builds submit . \
  --config=cloudbuild.yaml \
  --substitutions="_REGION=${REGION},_TAG=${TAG}" \
  --project="$PROJECT_ID" \
  --region="$REGION" \
  --default-buckets-behavior=regional-user-owned-bucket

echo ""
echo "Images pushed:"
echo "  ${REGION}-docker.pkg.dev/${PROJECT_ID}/kubelab/api:${TAG}"
echo "  ${REGION}-docker.pkg.dev/${PROJECT_ID}/kubelab/worker:${TAG}"
echo "  ${REGION}-docker.pkg.dev/${PROJECT_ID}/kubelab/frontend:${TAG}"
