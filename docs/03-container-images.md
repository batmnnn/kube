# Module 03 — Container Images

Kubernetes runs containers, not source code. This module covers building production-ready images and pushing to Google Artifact Registry.

## Concepts

### Image = Filesystem Snapshot + Metadata

A container image packages your app, runtime dependencies, and config. Kubernetes pulls images from a **registry** when scheduling pods.

### Multi-Stage Builds

Our Dockerfiles use two stages:

1. **Builder** — compiles Go binary (large toolchain)
2. **Runtime** — only the binary + minimal OS (small, secure)

See `app/api/Dockerfile` — final image is ~20MB vs ~800MB with full Go toolchain.

### Image Tags

| Tag | Use |
|-----|-----|
| `latest` | Development only — mutable, unpredictable |
| `v1.0.0` | Production — immutable semver |
| `git-sha` | CI/CD — traceable to exact commit |

**Never use `:latest` in production.**

## Build Locally

```bash
make build
docker images | grep kubelab
```

Test an image:

```bash
docker run --rm -p 8080:8080 \
  -e DB_HOST=host.docker.internal \
  kubelab-api:latest
```

Or use the full stack: `make dev`

## Push to Artifact Registry

Artifact Registry replaces Container Registry (GCR). Images live at:

```
REGION-docker.pkg.dev/PROJECT_ID/REPOSITORY/IMAGE:TAG
```

```bash
export GCP_PROJECT_ID=your-project-id
make push
```

The script:
1. Runs `gcloud auth configure-docker` for auth
2. Builds all three images (api, worker, frontend)
3. Pushes to `us-central1-docker.pkg.dev/PROJECT_ID/kubelab/`

Verify in Console: **Artifact Registry → kubelab**

## How K8s References Images

In `k8s/base/api/deployment.yaml`:

```yaml
containers:
  - name: api
    image: kubelab-api:latest
    imagePullPolicy: IfNotPresent  # IfNotPresent locally, Always in prod
```

Kustomize overlays rewrite image names for GKE:

```yaml
# k8s/overlays/gke-dev/kustomization.yaml
images:
  - name: kubelab-api
    newName: us-central1-docker.pkg.dev/PROJECT_ID/kubelab/api
    newTag: latest
```

## Security Best Practices (in our Dockerfiles)

- Run as non-root user (`USER app`)
- Minimal base image (Alpine)
- No secrets baked into images
- `HEALTHCHECK` for Docker; K8s uses probes instead

## Exercise

1. Build only the API: `docker build -t test-api app/api`
2. Inspect layers: `docker history test-api`
3. Push to Artifact Registry: `make push TAG=v0.1.0`
4. In GCP Console, verify all 3 images appear

## Next

→ [Module 04: Kubernetes Basics](04-kubernetes-basics.md)
