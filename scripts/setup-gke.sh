#!/usr/bin/env bash
# Creates a GKE cluster and Artifact Registry using Terraform.
# Prerequisites: gcloud CLI, terraform, authenticated GCP account
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
TF_DIR="$ROOT_DIR/infra/terraform"

echo "==> KubeLab GKE Setup"
echo ""

if ! command -v gcloud &>/dev/null; then
  echo "Error: gcloud CLI not found. Install: https://cloud.google.com/sdk/docs/install"
  exit 1
fi

if ! command -v terraform &>/dev/null; then
  echo "Error: terraform not found. Install: https://developer.hashicorp.com/terraform/install"
  exit 1
fi

PROJECT_ID="${GCP_PROJECT_ID:-}"
if [[ -z "$PROJECT_ID" ]]; then
  PROJECT_ID=$(gcloud config get-value project 2>/dev/null || true)
fi

if [[ -z "$PROJECT_ID" || "$PROJECT_ID" == "(unset)" ]]; then
  echo "Error: Set GCP_PROJECT_ID or run: gcloud config set project YOUR_PROJECT"
  exit 1
fi

echo "Using project: $PROJECT_ID"
echo ""

if [[ ! -f "$TF_DIR/terraform.tfvars" ]]; then
  echo "Creating terraform.tfvars from example..."
  cp "$TF_DIR/terraform.tfvars.example" "$TF_DIR/terraform.tfvars"
  sed -i.bak "s/your-gcp-project-id/$PROJECT_ID/" "$TF_DIR/terraform.tfvars"
  rm -f "$TF_DIR/terraform.tfvars.bak"
fi

cd "$TF_DIR"
terraform init
terraform plan -var="project_id=$PROJECT_ID"
echo ""
read -rp "Apply this plan? [y/N] " confirm
if [[ "$confirm" =~ ^[Yy]$ ]]; then
  terraform apply -var="project_id=$PROJECT_ID" -auto-approve
fi

echo ""
echo "==> Configuring kubectl"
eval "$(terraform output -raw get_credentials_command)"

echo ""
echo "==> Configuring Docker for Artifact Registry"
gcloud auth configure-docker "$(terraform output -raw artifact_registry_url | cut -d/ -f1)" --quiet

echo ""
echo "Setup complete!"
terraform output
