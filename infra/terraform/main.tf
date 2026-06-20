terraform {
  required_version = ">= 1.5"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.40"
    }
  }

  # Uncomment for remote state in production:
  # backend "gcs" {
  #   bucket = "your-terraform-state-bucket"
  #   prefix = "kubelab/gke"
  # }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

# Enable required GCP APIs
resource "google_project_service" "apis" {
  for_each = toset([
    "container.googleapis.com",
    "artifactregistry.googleapis.com",
    "compute.googleapis.com",
    "iam.googleapis.com",
    "cloudresourcemanager.googleapis.com",
  ])

  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

# Artifact Registry — stores container images (replaces GCR)
resource "google_artifact_registry_repository" "kubelab" {
  location      = var.region
  repository_id = "kubelab"
  description   = "KubeLab container images"
  format        = "DOCKER"

  depends_on = [google_project_service.apis]
}

# GKE cluster — Standard mode gives full control for learning
resource "google_container_cluster" "primary" {
  name     = var.cluster_name
  location = var.region

  # Remove default node pool immediately — we create a custom one below
  remove_default_node_pool = true
  initial_node_count       = 1

  network    = "default"
  subnetwork = "default"

  # Workload Identity — pods can authenticate to GCP APIs without JSON keys
  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  # Enable network policy support (required for our NetworkPolicy resources)
  network_policy {
    enabled = true
  }

  # Release channel for automatic upgrades on a stable cadence
  release_channel {
    channel = "REGULAR"
  }

  # Basic logging and monitoring (GKE integrates with Cloud Logging/Monitoring)
  logging_service    = "logging.googleapis.com/kubernetes"
  monitoring_service = "monitoring.googleapis.com/kubernetes"

  ip_allocation_policy {}

  depends_on = [google_project_service.apis]
}

resource "google_container_node_pool" "primary" {
  name       = "${var.cluster_name}-pool"
  location   = var.region
  cluster    = google_container_cluster.primary.name
  node_count = var.node_count

  node_config {
    machine_type = var.machine_type
    disk_size_gb = 50
    disk_type    = "pd-standard"

    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]

    labels = {
      env = var.environment
    }

    metadata = {
      disable-legacy-endpoints = "true"
    }

    workload_metadata_config {
      mode = "GKE_METADATA"
    }
  }

  management {
    auto_repair  = true
    auto_upgrade = true
  }
}

# Service account for Workload Identity demo (API pod accessing GCP resources)
resource "google_service_account" "kubelab_api" {
  account_id   = "kubelab-api"
  display_name = "KubeLab API Workload Identity"
}

resource "google_service_account_iam_member" "workload_identity" {
  service_account_id = google_service_account.kubelab_api.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[kubelab/kubelab-api]"
}

# Grant read access to Artifact Registry for pulling images
resource "google_project_iam_member" "artifact_reader" {
  project = var.project_id
  role    = "roles/artifactregistry.reader"
  member  = "serviceAccount:${data.google_project.project.number}-compute@developer.gserviceaccount.com"
}

data "google_project" "project" {
  project_id = var.project_id
}
