#!/usr/bin/env bash
# Load locally built images into a kind cluster for offline K8s testing.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
CLUSTER_NAME="${KIND_CLUSTER_NAME:-kubelab}"

if ! command -v kind &>/dev/null; then
  echo "Install kind: https://kind.sigs.k8s.io/docs/user/quick-start/"
  exit 1
fi

if ! kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  echo "Creating kind cluster: $CLUSTER_NAME"
  kind create cluster --name "$CLUSTER_NAME"
fi

echo "Building images..."
docker build -t kubelab-api:local "$ROOT_DIR/app/api"
docker build -t kubelab-worker:local "$ROOT_DIR/app/worker"
docker build -t kubelab-frontend:local "$ROOT_DIR/app/frontend"

echo "Loading into kind..."
kind load docker-image kubelab-api:local --name "$CLUSTER_NAME"
kind load docker-image kubelab-worker:local --name "$CLUSTER_NAME"
kind load docker-image kubelab-frontend:local --name "$CLUSTER_NAME"

echo "Deploying local overlay..."
kubectl apply -k "$ROOT_DIR/k8s/overlays/local"

echo ""
echo "Port-forward to access locally:"
echo "  kubectl port-forward svc/frontend -n kubelab 3000:80"
