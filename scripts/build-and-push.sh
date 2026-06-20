#!/usr/bin/env bash
# Build all container images and push to Google Artifact Registry.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

PROJECT_ID="${GCP_PROJECT_ID:-$(gcloud config get-value project 2>/dev/null)}"
REGION="${GCP_REGION:-us-central1}"
TAG="${IMAGE_TAG:-latest}"
REGISTRY="${REGION}-docker.pkg.dev/${PROJECT_ID}/kubelab"

if [[ -z "$PROJECT_ID" || "$PROJECT_ID" == "(unset)" ]]; then
  echo "Error: Set GCP_PROJECT_ID or configure gcloud project"
  exit 1
fi

echo "==> Building and pushing images to $REGISTRY"
gcloud auth configure-docker "${REGION}-docker.pkg.dev" --quiet 2>/dev/null || true

build_and_push() {
  local name=$1
  local context=$2
  local image="${REGISTRY}/${name}:${TAG}"

  echo "--- Building $name"
  docker build -t "$image" "$context"
  echo "--- Pushing $image"
  docker push "$image"
}

# Generate go.sum if missing
if [[ ! -f "$ROOT_DIR/app/api/go.sum" ]]; then
  echo "Generating go.sum for api..."
  (cd "$ROOT_DIR/app/api" && go mod tidy)
fi
if [[ ! -f "$ROOT_DIR/app/worker/go.sum" ]]; then
  echo "Generating go.sum for worker..."
  (cd "$ROOT_DIR/app/worker" && go mod tidy)
fi

build_and_push "api" "$ROOT_DIR/app/api"
build_and_push "worker" "$ROOT_DIR/app/worker"
build_and_push "frontend" "$ROOT_DIR/app/frontend"

echo ""
echo "Images pushed:"
echo "  ${REGISTRY}/api:${TAG}"
echo "  ${REGISTRY}/worker:${TAG}"
echo "  ${REGISTRY}/frontend:${TAG}"
