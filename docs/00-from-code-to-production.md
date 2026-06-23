# From Code to Production — Explained for Beginners

You wrote Go and HTML. Now it's running on Google Cloud with Kubernetes.

This doc explains **the whole journey in plain English** — no prior cloud experience needed. Read top to bottom once. Come back to any section when you're stuck.

---

## Before we start: 3 ideas only

If you remember just these, everything else clicks later.

| Idea | Plain English |
|------|----------------|
| **Container** | Your app packed in a box that runs the same everywhere |
| **Kubernetes (K8s)** | A robot manager that keeps those boxes running on servers |
| **Deploy** | "Here's the recipe — please run my app like this" |

That's it for now. We'll fill in the details.

---

## The whole journey (30-second version)

```
You write code on your laptop
        ↓
Docker packs it into an IMAGE (a saved box)
        ↓
Image gets uploaded to ARTIFACT REGISTRY (Google's storage for boxes)
        ↓
You tell Kubernetes: "Run this image" (YAML files)
        ↓
GKE starts your app on Google's servers
        ↓
User opens a URL in the browser → your app answers
```

**Your repo automates this.** When you `git push` to `main`, GitHub Actions does the build + upload + deploy for you.

---

## What app are we even running?

KubeLab is an **order app** — like a tiny Amazon checkout.

| Piece | What it does | Built by you? |
|-------|--------------|---------------|
| **frontend** | Website you see in the browser (HTML + nginx) | ✅ Yes |
| **api** | Backend that handles `/api/orders` (Go) | ✅ Yes |
| **worker** | Background job that processes orders (Go) | ✅ Yes |
| **postgres** | Database — stores orders | ❌ Uses ready-made image |
| **redis** | Queue — holds jobs for the worker | ❌ Uses ready-made image |

Think of it like a restaurant:

- **frontend** = dining room (what customers see)
- **api** = waiter (takes orders)
- **worker** = kitchen (cooks in the back)
- **postgres** = pantry (stores ingredients long-term)
- **redis** = order tickets on the rail (short-term queue)

All of this runs inside one **namespace** called `kubelab` — like one floor of a building dedicated to your app.

---

## Part 1 — Packing your app (Docker)

### Problem Docker solves

> "It works on my laptop but not on the server."

Your laptop has Go 1.22, certain folders, certain settings. The server might not.

**Docker's fix:** pack the app + everything it needs into one **image**. Run that image anywhere. Same result.

### Image vs container

| Term | Think of it as… |
|------|------------------|
| **Image** | A recipe / template (frozen) |
| **Container** | The actual running app (live) |

One image → many containers (if you scale to 5 copies, you run the same image 5 times).

### What's in your Dockerfiles?

You have three, one per app:

- `app/api/Dockerfile`
- `app/worker/Dockerfile`
- `app/frontend/Dockerfile`

**API example (simplified):**

```
Step 1: Use a Go image → compile your code into one binary
Step 2: Use a tiny Alpine Linux image → copy only the binary in
Step 3: Run as non-root user → safer
Step 4: Start the app on port 8080
```

**Why two steps (multi-stage build)?**  
Step 1 needs compilers (heavy). Step 2 only needs the finished app (light). Smaller image = faster upload + less to break.

**Frontend** is simpler: take nginx (web server), copy your HTML/CSS/JS, done.

### Try it yourself (optional)

```bash
cd app/api
docker build -t my-api:test .
docker run -p 8080:8080 my-api:test
# visit http://localhost:8080/health
```

---

## Part 2 — Storing images (Artifact Registry)

### Problem

You built a box (image) on your laptop. **GKE servers need a copy.** Where do you put it?

**Answer: Artifact Registry** — Google's private Docker storage.

### The address format

Every image has a full address:

```
us-central1-docker.pkg.dev/learning-deplo/kubelab/api:abc123
│            │            │           │      │   │
│            │            │           │      │   └── tag (version name)
│            │            │           │      └── image name
│            │            │           └── repo name (kubelab)
│            │            └── your GCP project ID
│            └── region
```

- **Tag `abc123`** = usually your git commit ID (so you know exactly what's running)
- **Tag `latest`** = "most recent" (handy for manual testing)

### Who uploads? Who downloads?

| Who | Action |
|-----|--------|
| **GitHub Actions** (on push) | Builds + uploads |
| **Cloud Shell** (`cloud-shell-build.sh`) | Builds + uploads |
| **GKE nodes** | Download (pull) when starting a pod |

You created the registry once with `cloud-shell-setup.sh`. After that, CI keeps pushing new versions.

---

## Part 3 — Running apps (GKE + Kubernetes)

### What is GKE?

**Google Kubernetes Engine** = Kubernetes hosted by Google.

Google runs the **control plane** (the brain). You get **worker nodes** (machines that run your containers).

You don't SSH into nodes usually. You talk to the brain with **`kubectl`**:

```bash
kubectl get pods -n kubelab    # "show me running apps"
```

### The 4 Kubernetes words you MUST know

#### 1. Pod

The **smallest unit** — usually one container running your app.

```
Pod = one running copy of your api (or frontend, etc.)
```

Pods die and get recreated. Don't get attached to a pod name.

#### 2. Deployment

**"Keep N copies of this pod running."**

If a pod crashes, Deployment starts a new one.  
If you update the image, Deployment does a **rolling update** (swap old → new).

File: `k8s/base/api/deployment.yaml`

#### 3. Service

Pods get random IPs. A **Service** gives a **stable name** inside the cluster.

```
api  → always points to healthy api pods
postgres  → always points to postgres
```

Other pods call `http://api:8080` — Kubernetes DNS finds the right pod.

File: `k8s/base/api/service.yaml`

#### 4. Ingress

**The public front door.** Gives you an external IP so browsers on the internet can reach your app.

File: `k8s/base/ingress/ingress.yaml`

Routes:

- `/` → frontend
- `/api` → api

Getting the IP takes **5–15 minutes** the first time. Be patient.

### Other files (don't memorize — just know they exist)

| File kind | Why it's there |
|-----------|----------------|
| **ConfigMap** | Settings like `DB_HOST=postgres` (not secret) |
| **Secret** | Passwords |
| **StatefulSet** | Postgres — needs permanent disk |
| **Init container** | "Wait for database before starting api" |
| **Probes** | K8s checks `/health` — is the app alive? ready for traffic? |
| **HPA** | Auto-scale api if CPU goes up |
| **NetworkPolicy** | Firewall between pods |

All live under `k8s/base/`.

---

## Part 4 — How YAML gets to the cluster

You don't edit 20 files by hand each deploy. You use **Kustomize**.

### Simple idea

```
k8s/base/          ← shared templates (same for everyone)
k8s/overlays/
   gke-dev/        ← tweaks for your trial cluster
   local/          ← tweaks for laptop testing
```

**gke-dev overlay** does things like:

- Set replicas to **1** (save money on small cluster)
- Point images to **Artifact Registry** instead of local names
- Faster rollouts for CI

### What `deploy.sh` does (4 steps)

```bash
./scripts/deploy.sh gke-dev
```

1. **Build** the final YAML from kustomize  
2. **Replace** placeholders: `PROJECT_ID`, `REGION`, `IMAGE_TAG`  
3. **Apply** to cluster: `kubectl apply -f ...`  
4. **Wait** for pods to come up  

**`kubectl apply`** = "Make the cluster look like this file."  
Safe to run many times.

**`IMAGE_TAG`** links build to deploy:

```
CI builds image tagged abc123  →  deploy sets IMAGE_TAG=abc123  →  cluster pulls abc123
```

---

## Part 5 — How traffic reaches your app

User types `http://35.x.x.x/api/orders` in Chrome.

```
Browser
   ↓
Google Load Balancer (created by Ingress)
   ↓
Your api pod (port 8080)
   ↓
postgres pod (database)
   ↓
Response back to browser
```

### Inside the cluster (pod talking to pod)

Frontend nginx can also call the api:

```
frontend pod  →  http://api:8080  →  api pod
```

The name **`api`** works because of **Service + DNS**. Magic for beginners: just use the service name.

### Why so many networking files?

GKE is picky about load balancers. Extra annotations connect Kubernetes Services to Google's network:

- **NEG** — load balancer talks directly to pod IPs
- **BackendConfig** — tells Google how to health-check (`/health` on port 8080)

That's why `k8s/base/api/service.yaml` has extra `cloud.google.com/...` lines. Without them, Ingress often breaks on GKE.

---

## Part 6 — What happens when you `git push`

Your CI file: `.github/workflows/ci.yaml`

### Job 1: Test

- Compiles Go code (does it build?)
- Checks Kubernetes YAML renders (typos?)

### Job 2: Build & push

1. GitHub proves its identity to Google (**Workload Identity** — no password file stored)
2. Builds 3 Docker images on GitHub's machine
3. Pushes to Artifact Registry with tag = git commit SHA

### Job 3: Deploy

1. Connects to your GKE cluster
2. Runs `./scripts/deploy.sh gke-dev`
3. Waits until api, worker, frontend pods are healthy
4. Runs a quick **smoke test** (curl health endpoints from inside cluster)

**Total time:** roughly 3–7 minutes.

### Workload Identity in one sentence

> GitHub says "I'm repo batmnnn/kube" → Google checks a trust rule → gives temporary permission to act as `github-ci@...` service account.

Setup once: `./scripts/setup-github-cicd.sh`  
Secrets in GitHub: `GCP_PROJECT_ID`, `GKE_CLUSTER`, `WIF_PROVIDER`, `WIF_SERVICE_ACCOUNT`

---

## Part 7 — What happens when you deploy a new version

You changed code. New image exists. `deploy.sh` runs.

**Rolling update (simple version):**

```
1. Start new pod with new image
2. Wait until it's healthy (probes pass)
3. Stop old pod
4. Done
```

On a **small dev cluster**, we use fast settings (replace old pod quickly). On production you'd keep zero downtime (new up before old down).

**If deploy feels stuck:** old pod might be waiting for the load balancer to finish draining connections. Check:

```bash
kubectl get pods -n kubelab
kubectl rollout status deployment/api -n kubelab
```

---

## Two ways to build (you have both)

| Method | When you use it |
|--------|-----------------|
| **GitHub Actions** | Every push to main (automatic) |
| **Cloud Build** (`cloud-shell-build.sh`) | Manual builds from Cloud Shell |

Both push to the **same registry**. Kubernetes doesn't care who built the image.

---

## Cheat sheet — commands you'll actually use

```bash
# See running apps
kubectl get pods -n kubelab

# See more detail on a pod
kubectl describe pod <pod-name> -n kubelab

# Read app logs
kubectl logs -l app.kubernetes.io/name=api -n kubelab

# Get public URL
kubectl get ingress kubelab-ingress -n kubelab

# Deploy manually (after building images)
export GCP_PROJECT_ID=learning-deplo
export IMAGE_TAG=latest
./scripts/deploy.sh gke-dev
```

---

## When something breaks — start here

| What you see | First thing to check |
|--------------|----------------------|
| `ImagePullBackOff` | Wrong image tag? Image not pushed? Run `kubectl describe pod ...` |
| `CrashLoopBackOff` | App crashing — run `kubectl logs ...` |
| Pod stuck `Init:0/2` | Database not ready — check postgres pod |
| Ingress has no IP | Wait 10 min, then `kubectl describe ingress ...` |
| 502 Bad Gateway | App not healthy — check probes and logs |
| CI auth failed | GitHub secrets wrong — see `docs/10-cicd.md` |

**Golden rule:** read `kubectl describe pod` Events at the bottom. Kubernetes usually tells you what's wrong.

---

## Quick quiz (test yourself)

1. What's the difference between an image and a container?  
2. Where are your Docker images stored in GCP?  
3. What Kubernetes object gives you a public IP?  
4. What does `IMAGE_TAG` connect together?  
5. Why do we have init containers on the api?

<details>
<summary>Answers</summary>

1. Image = template; container = running instance.  
2. Artifact Registry — `us-central1-docker.pkg.dev/PROJECT/kubelab/...`  
3. Ingress  
4. The image CI built ↔ the image Deployment pulls  
5. So api waits for postgres/redis before starting (avoid crash loops).

</details>

---

## What to read next

| Want to learn… | Open this |
|----------------|-----------|
| Cloud Shell deploy steps | [cloud-shell-deploy.md](cloud-shell-deploy.md) |
| CI/CD setup | [10-cicd.md](10-cicd.md) |
| kubectl basics | [04-kubernetes-basics.md](04-kubernetes-basics.md) |
| Networking deep dive | [05-networking.md](05-networking.md) |
| Full module list | [README.md](../README.md) |

---

## One page to screenshot

```
CODE (app/api, app/worker, app/frontend)
  → Dockerfile builds IMAGE
  → push to ARTIFACT REGISTRY
  → k8s YAML says "run this image"
  → GKE creates PODS
  → SERVICE gives stable name
  → INGRESS gives public IP
  → USER visits website

git push main → GitHub Actions does all of this automatically
```

You don't need to understand every YAML field on day one.  
**Learn the flow first.** Then open one file at a time and ask: *"Where does this fit in the diagram?"*

That’s how working engineers learn too.
