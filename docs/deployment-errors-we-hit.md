# Errors We Hit — And How We Fixed Them

A running log of real failures from deploying KubeLab to GKE with GitHub Actions CI/CD. Use this when something breaks and the error looks familiar.

**Stack:** GKE · Artifact Registry · GitHub Actions · Workload Identity Federation · Kustomize

---

## 1. Where does `WIF_PROVIDER` come from?

### Symptom

Setting up GitHub secrets — unclear what value to paste for `WIF_PROVIDER`.

### Cause

It's not in the GCP console by default. It's created by the CI setup script and is built from your **project number**, not project ID.

### Fix

Run once in Cloud Shell:

```bash
export GCP_PROJECT_ID=YOUR_PROJECT_ID
export GITHUB_REPO=batmnnn/kube
./scripts/setup-github-cicd.sh
```

Copy the printed line:

```
WIF_PROVIDER = projects/663804652181/locations/global/workloadIdentityPools/github-pool/providers/github-provider
```

Or fetch manually:

```bash
gcloud iam workload-identity-pools providers describe github-provider \
  --location=global \
  --workload-identity-pool=github-pool \
  --format='value(name)'
```

---

## 2. `cloudbuild.yaml` YAML parse error

### Symptom

```
ERROR: (gcloud.builds.submit) parsing cloudbuild.yaml: while parsing a flow sequence
  in "cloudbuild.yaml", line 36, column 11
expected ',' or ']', but got '{'
```

CI failed on **Build & Push Images** before any GCP auth issue.

### Cause

Push steps used inline YAML arrays with substitution variables:

```yaml
# BROKEN — { in ${...} breaks YAML flow syntax
args: [push, ${_REGION}-docker.pkg.dev/${PROJECT_ID}/kubelab/api:${_TAG}]
```

### Fix

Use block-style `args` (same as build steps):

```yaml
args:
  - push
  - ${_REGION}-docker.pkg.dev/${PROJECT_ID}/kubelab/api:${_TAG}
```

**Commit:** `Fix cloudbuild.yaml YAML parse error in push steps.`

---

## 3. `Invalid bucket name ..._cloudbuild`

### Symptom

```
ERROR: (gcloud.builds.submit) HTTPError 400: Invalid bucket name: '***_cloudbuild'
```

### Cause

Usually one of:

- Malformed `GCP_PROJECT_ID` GitHub secret (spaces, newlines, wrong value like WIF path)
- Cloud Build trying to auto-create a default staging bucket with a bad name

### Fix

- Validate secret is **project ID only** (e.g. `learning-deplo`), not project number
- Trim whitespace in CI before use
- Later: CI switched away from `gcloud builds submit` entirely (see #4)

---

## 4. `Forbidden from accessing the bucket` (Cloud Build staging)

### Symptom

```
ERROR: (gcloud.builds.submit) The user is forbidden from accessing the bucket
[***_cloudbuild] ... serviceusage.services.use permission
```

Or with regional buckets:

```
... forbidden from accessing the bucket [***_us-central1_cloudbuild]
```

### Cause

GitHub Actions authenticates as `github-ci@...` service account. That identity did not have permission to create/use Cloud Build's GCS staging bucket.

### Fix (what we did)

**Option A — CI bypass (chosen):** Build Docker images on the GitHub runner and push directly to Artifact Registry. No GCS bucket needed.

**Option B — IAM (for Cloud Shell builds):** Re-run `./scripts/setup-github-cicd.sh` which now grants:

- `roles/storage.admin`
- `roles/serviceusage.serviceUsageConsumer`
- Explicit bucket IAM on `{project}_cloudbuild`

**Commit:** `Build CI images with Docker instead of Cloud Build.`

---

## 5. `artifactregistry.tags.delete` permission denied

### Symptom

Build succeeded, **Tag images as latest** step failed:

```
ERROR: (gcloud.artifacts.docker.tags.add) PERMISSION_DENIED:
Permission 'artifactregistry.tags.delete' denied on resource '.../tags/latest'
```

### Cause

`gcloud artifacts docker tags add` to move `latest` requires **delete** permission on the old tag. `artifactregistry.writer` can push but not retag that way.

### Fix

Tag and push `latest` with Docker instead:

```bash
docker build -t REGISTRY/api:SHA -t REGISTRY/api:latest .
docker push REGISTRY/api:SHA
docker push REGISTRY/api:latest
```

**Commit:** `Tag latest images via docker push instead of gcloud.`

---

## 6. `deploy.sh` — `sed: unterminated 's' command`

### Symptom

CI **Deploy to GKE** failed:

```
sed: -e expression #1, char 27: unterminated `s' command
```

### Cause

`sed "s/PROJECT_ID/${PROJECT_ID}/g"` breaks when the replacement contains `/` characters (e.g. if secret had a path-like value), or special sed characters.

### Fix

- Use pipe delimiters: `sed "s|PROJECT_ID|${PROJECT_ID}|g"`
- Trim whitespace from `GCP_PROJECT_ID` before deploy

**Commit:** `Fix deploy.sh sed substitution for GCP project IDs.`

---

## 7. RBAC — `cannot patch resource "roles"`

### Symptom

Deploy mostly succeeded, then:

```
roles.rbac.authorization.k8s.io "kubelab-api-reader" is forbidden:
User "***" cannot patch resource "roles" ... requires container.roles.update
```

Same for `rolebindings`.

### Cause

`github-ci@` had `roles/container.developer`. That role can deploy apps but **not** RBAC Roles/RoleBindings in the manifest.

### Fix

Upgrade CI service account to `roles/container.admin`:

```bash
gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
  --member="serviceAccount:github-ci@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/container.admin"
```

Or re-run `./scripts/setup-github-cicd.sh` (script updated to use `container.admin`).

**Commit:** `Grant container.admin to CI service account for deploy.`

---

## 8. Rollout timeout — old replicas pending termination

### Symptom

```
Waiting for deployment "api" rollout to finish: 1 old replicas are pending termination...
error: timed out waiting for the condition
```

### Cause

On a **small dev cluster** (1 replica):

- Base manifest used `maxUnavailable: 0` + `maxSurge: 1` (zero-downtime)
- GKE Ingress **NEG** connection draining defaulted to **60 seconds**
- Old pod sat in `Terminating` while load balancer drained connections → CI 180s timeout

### Fix

`gke-dev` overlay patches:

- `maxUnavailable: 1`, `maxSurge: 0` (replace in place — OK for dev)
- `terminationGracePeriodSeconds: 15`
- `connectionDraining.drainingTimeoutSec: 10`
- PDB relaxed (`minAvailable: 0`)
- CI rollout timeout increased to 300s with diagnostics

**Commit:** `Speed up gke-dev rollouts for CI on small clusters.`

---

## 9. Local Docker — `unknown driver "postgres"`

### Symptom

`docker compose up --build`:

```
api-1  | database connection failed ... sql: unknown driver "postgres" (forgotten import?)
api-1 exited with code 1
frontend-1 | host not found in upstream "api"
```

### Cause

Rewriting `app/api/main.go` dropped the blank import for the Postgres driver:

```go
_ "github.com/lib/pq"   // required for database/sql
```

Frontend failed **because** API never started (nginx resolves `api` at startup).

### Fix

Re-add `_ "github.com/lib/pq"` to `app/api/main.go`.

---

## 10. GitHub secret mistakes (general)

### Symptoms

Various auth and bucket errors with `***` masked in logs.

### Common mistakes

| Wrong secret value | What happens |
|--------------------|--------------|
| Project **number** instead of ID | Weird bucket names, sed errors |
| Full WIF provider path in `GCP_PROJECT_ID` | sed / bucket errors |
| Trailing newline or space in secret | Invalid bucket name, auth quirks |
| Old project's WIF secrets after new GCP account | Auth succeeds against wrong project or fails |

### Fix

All four secrets must match **one** GCP project:

| Secret | Format |
|--------|--------|
| `GCP_PROJECT_ID` | `kubelab-dev-2026` (ID only) |
| `GKE_CLUSTER` | `kubelab-cluster` |
| `WIF_PROVIDER` | `projects/NUM/locations/global/workloadIdentityPools/github-pool/providers/github-provider` |
| `WIF_SERVICE_ACCOUNT` | `github-ci@PROJECT_ID.iam.gserviceaccount.com` |

Re-run `./scripts/setup-github-cicd.sh` after switching accounts.

---

## Timeline of fixes (commits)

| Order | Commit theme | Error addressed |
|-------|--------------|-----------------|
| 1 | Fix cloudbuild.yaml YAML | #2 Parse error |
| 2 | Cloud Build staging bucket / validation | #3 Invalid bucket |
| 3 | Docker build in CI | #4 Bucket forbidden |
| 4 | docker push for `:latest` | #5 Tag permission |
| 5 | deploy.sh sed pipes | #6 Sed error |
| 6 | container.admin for CI | #7 RBAC patch |
| 7 | gke-dev rollout patches | #8 Rollout timeout |

---

## Quick “what failed?” map

| CI step fails at… | Look up |
|-------------------|---------|
| `gcloud builds submit` | #2, #3, #4 |
| `Tag images as latest` | #5 |
| `Deploy manifests` (sed) | #6 |
| `Deploy manifests` (Forbidden roles) | #7 |
| `Verify rollouts` | #8 |
| `Authenticate to Google Cloud` | #10, #1 |
| Local `docker compose` api crash | #9 |

---

## Related docs

- [cloud-shell-deploy.md](cloud-shell-deploy.md) — manual deploy path
- [10-cicd.md](10-cicd.md) — CI/CD setup and troubleshooting
- [00-from-code-to-production.md](00-from-code-to-production.md) — beginner deployment story

---

*Last updated after WordRush app changes and new-account redeploy guide.*
