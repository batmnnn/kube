.PHONY: help dev build push deploy setup-gke local-k8s tidy test clean

PROJECT_ID ?= $(shell gcloud config get-value project 2>/dev/null)
REGION     ?= us-central1
OVERLAY    ?= gke-dev
TAG        ?= latest

help:
	@echo "KubeLab — GKE Learning Project"
	@echo ""
	@echo "  make dev          Run locally with Docker Compose"
	@echo "  make tidy         Generate go.sum files"
	@echo "  make build        Build container images locally"
	@echo "  make push         Build and push to Artifact Registry"
	@echo "  make setup-gke    Create GKE cluster with Terraform"
	@echo "  make deploy       Deploy to GKE (OVERLAY=gke-dev|gke-prod)"
	@echo "  make local-k8s    Deploy to local kind cluster"
	@echo "  make status       Show cluster status"
	@echo "  make logs-api     Tail API logs"
	@echo "  make clean        Remove local Docker images"

dev:
	docker compose up --build

tidy:
	cd app/api && go mod tidy
	cd app/worker && go mod tidy

build: tidy
	docker build -t kubelab-api:$(TAG) app/api
	docker build -t kubelab-worker:$(TAG) app/worker
	docker build -t kubelab-frontend:$(TAG) app/frontend

push:
	GCP_PROJECT_ID=$(PROJECT_ID) GCP_REGION=$(REGION) IMAGE_TAG=$(TAG) ./scripts/build-and-push.sh

setup-gke:
	GCP_PROJECT_ID=$(PROJECT_ID) ./scripts/setup-gke.sh

deploy:
	GCP_PROJECT_ID=$(PROJECT_ID) GCP_REGION=$(REGION) ./scripts/deploy.sh $(OVERLAY)

local-k8s:
	./scripts/local-k8s.sh

status:
	kubectl get all,ingress,hpa,pdb -n kubelab

logs-api:
	kubectl logs -l app.kubernetes.io/name=api -n kubelab -f --tail=100

clean:
	docker rmi kubelab-api:$(TAG) kubelab-worker:$(TAG) kubelab-frontend:$(TAG) 2>/dev/null || true
