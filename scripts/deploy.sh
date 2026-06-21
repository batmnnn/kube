#!/usr/bin/env bash
# Deploy KubeLab to GKE using Kustomize overlays.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

OVERLAY="${1:-gke-dev}"
PROJECT_ID="${GCP_PROJECT_ID:-$(gcloud config get-value project 2>/dev/null)}"
REGION="${GCP_REGION:-us-central1}"
IMAGE_TAG="${IMAGE_TAG:-latest}"

PROJECT_ID="$(printf '%s' "$PROJECT_ID" | tr -d '[:space:]')"
REGION="$(printf '%s' "$REGION" | tr -d '[:space:]')"
IMAGE_TAG="$(printf '%s' "$IMAGE_TAG" | tr -d '[:space:]')"

if [[ -z "$PROJECT_ID" || "$PROJECT_ID" == "(unset)" ]]; then
  echo "Error: Set GCP_PROJECT_ID or configure gcloud project"
  exit 1
fi

OVERLAY_DIR="$ROOT_DIR/k8s/overlays/$OVERLAY"
if [[ ! -d "$OVERLAY_DIR" ]]; then
  echo "Error: Overlay '$OVERLAY' not found at $OVERLAY_DIR"
  echo "Available: local, gke-dev, gke-prod"
  exit 1
fi

echo "==> Deploying KubeLab (overlay: $OVERLAY)"
echo "    Project: $PROJECT_ID | Region: $REGION | Tag: $IMAGE_TAG"
echo ""

# Build kustomize output with project/region substitution
TMP_DIR=$(mktemp -d)
if command -v kustomize &>/dev/null; then
  KUSTOMIZE=(kustomize build)
else
  KUSTOMIZE=(kubectl kustomize)
fi

"${KUSTOMIZE[@]}" "$OVERLAY_DIR" \
  | sed "s|PROJECT_ID|${PROJECT_ID}|g" \
  | sed "s|REGION|${REGION}|g" \
  | sed "s|IMAGE_TAG|${IMAGE_TAG}|g" \
  > "$TMP_DIR/manifests.yaml"

echo "--- Preview (first 30 lines)"
head -30 "$TMP_DIR/manifests.yaml"
echo "..."
echo ""

kubectl apply -f "$TMP_DIR/manifests.yaml"

echo ""
echo "==> Waiting for rollouts"
kubectl rollout status deployment/api -n kubelab --timeout=120s || true
kubectl rollout status deployment/frontend -n kubelab --timeout=120s || true
kubectl rollout status deployment/worker -n kubelab --timeout=120s || true
kubectl rollout status statefulset/postgres -n kubelab --timeout=180s || true

echo ""
echo "==> Pod status"
kubectl get pods -n kubelab

echo ""
echo "==> Services and Ingress"
kubectl get svc,ingress -n kubelab

echo ""
INGRESS_IP=""
for i in $(seq 1 30); do
  INGRESS_IP=$(kubectl get ingress kubelab-ingress -n kubelab -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || true)
  if [[ -n "$INGRESS_IP" ]]; then
    break
  fi
  echo "Waiting for Ingress IP... ($i/30)"
  sleep 10
done

if [[ -n "$INGRESS_IP" ]]; then
  echo ""
  echo "App available at: http://${INGRESS_IP}"
  echo "(GKE Ingress can take 5-10 minutes to fully provision)"
else
  echo ""
  echo "Ingress IP not yet assigned. Check with:"
  echo "  kubectl get ingress kubelab-ingress -n kubelab -w"
fi

rm -rf "$TMP_DIR"
