# Deploy from Google Cloud Shell Only

Everything runs in the browser — no local Docker, Terraform, or gcloud install needed.

Cloud Shell includes: `gcloud`, `kubectl`, `git`, `docker` (limited).  
We use **gcloud** for the cluster and **Cloud Build** for images instead.

## Step 0 — Open Cloud Shell

1. Go to [console.cloud.google.com](https://console.cloud.google.com)
2. Click the **Cloud Shell** icon (terminal) top-right
3. Set your project:

```bash
gcloud config set project YOUR_PROJECT_ID
export GCP_PROJECT_ID=$(gcloud config get-value project)
export GCP_REGION=us-central1
```

Billing must be enabled on the project.

## Step 1 — Get the code into Cloud Shell

**Option A — Git (recommended)**

Push this repo to GitHub, then in Cloud Shell:

```bash
git clone https://github.com/YOU/kube.git
cd kube
```

**Option B — Upload**

1. Zip the `kube` folder on your Mac
2. In Cloud Shell: **⋮ menu → Upload**
3. Upload the zip, then:

```bash
unzip kube.zip && cd kube
```

## Step 2 — Create cluster + registry (~10 min)

```bash
./scripts/cloud-shell-setup.sh
```

Creates Artifact Registry, GKE cluster ( **1 node** by default), configures kubectl.

```bash
# optional overrides
export GKE_NODE_COUNT=1
export GKE_MACHINE_TYPE=e2-medium
```

## Step 3 — Build images with Cloud Build (~5 min)

```bash
./scripts/cloud-shell-build.sh
```

No local Docker — Google builds the images in the cloud.

## Step 4 — Deploy to Kubernetes

```bash
./scripts/deploy.sh gke-dev
```

## Step 5 — Open the app

```bash
kubectl get ingress kubelab-ingress -n kubelab -w
```

When `ADDRESS` appears, open `http://THAT_IP` in your browser.

Test:

```bash
curl http://INGRESS_IP/api/scores
```

## Teardown (stop charges)

```bash
./scripts/cloud-shell-teardown.sh
```

## Troubleshooting

| Problem | Fix |
|---------|-----|
| `ImagePullBackOff` | Re-run `./scripts/cloud-shell-build.sh` then `./scripts/deploy.sh gke-dev` |
| No Ingress IP | Wait 10 min; `kubectl describe ingress kubelab-ingress -n kubelab` |
| Cloud Build permission denied | Re-run setup script (grants Cloud Build push access) |
| `Forbidden` on cluster create | Enable billing; need Owner/Editor role |

## What each script does

| Script | Purpose |
|--------|---------|
| `cloud-shell-setup.sh` | APIs, Artifact Registry, GKE cluster |
| `cloud-shell-build.sh` | Cloud Build → push 3 images |
| `deploy.sh gke-dev` | Apply all Kubernetes manifests |
| `cloud-shell-teardown.sh` | Delete cluster |
