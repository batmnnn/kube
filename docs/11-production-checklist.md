# Module 11 — Production Checklist

What changes when you go from learning to running real workloads on GKE.

## Security

- [ ] **HTTPS everywhere** — GKE ManagedCertificate + Ingress TLS
- [ ] **No secrets in git** — External Secrets Operator + GCP Secret Manager
- [ ] **NetworkPolicies enforced** — already in this project; verify with tests
- [ ] **Pod Security Standards** — namespace set to `restricted` in production
- [ ] **Non-root containers** — already configured in our Dockerfiles
- [ ] **Artifact Registry vulnerability scanning** — enable in GCP Console
- [ ] **RBAC least privilege** — audit with `kubectl auth can-i --list`
- [ ] **Private GKE cluster** — nodes without public IPs for sensitive workloads

### Enable HTTPS

```yaml
# Add to ingress annotations:
networking.gke.io/managed-certificates: kubelab-cert
kubernetes.io/ingress.allow-http: "false"
```

## Reliability

- [ ] **Multi-zone node pool** — spread nodes across zones
- [ ] **PDB on all critical Deployments** — API has one; add for frontend
- [ ] **HPA with sensible limits** — tune min/max based on traffic patterns
- [ ] **Database backups** — Cloud SQL with automated backups, or Velero for PVCs
- [ ] **Disaster recovery runbook** — document restore procedure

## Observability

- [ ] **Alerting** — Cloud Monitoring alerts on pod restarts, high error rate
- [ ] **SLOs/SLIs** — define uptime target (e.g., 99.9%)
- [ ] **Distributed tracing** — OpenTelemetry + Cloud Trace
- [ ] **Log retention policy** — set in Cloud Logging

## Cost Optimization

- [ ] **Right-size nodes** — start with e2-medium, monitor with `kubectl top`
- [ ] **Consider Autopilot** — pay per pod, no idle node cost
- [ ] **Preemptible/Spot nodes** — for non-critical workloads (worker is a candidate)
- [ ] **Cluster autoscaling** — add nodes when pending pods can't schedule
- [ ] **Delete dev clusters** — `terraform destroy` when not learning

```bash
# Enable cluster autoscaling (example)
gcloud container clusters update kubelab-cluster \
  --enable-autoscaling \
  --min-nodes=1 \
  --max-nodes=5 \
  --region us-central1
```

## Operations

- [ ] **Runbooks** for common incidents (pod crash loop, DB full, Ingress down)
- [ ] **Change management** — deploy during maintenance windows for prod
- [ ] **Image immutability** — semver or SHA tags only, never `:latest`
- [ ] **Regular cluster upgrades** — GKE release channel handles this; monitor

## Managed Services vs In-Cluster

| Component | Learning (this project) | Production recommendation |
|-----------|------------------------|---------------------------|
| Database | PostgreSQL StatefulSet | **Cloud SQL** (managed, HA, backups) |
| Cache | Redis Deployment | **Memorystore for Redis** |
| Ingress | GKE Ingress | Same, plus Cloud CDN |
| Secrets | K8s Secret | **Secret Manager** + External Secrets |
| Monitoring | Basic metrics | **Cloud Monitoring** + Managed Prometheus |

## Final Exercise — Full Deployment Review

Run through this checklist on your deployed cluster:

```bash
# 1. All pods healthy?
kubectl get pods -n kubelab

# 2. Ingress working?
curl http://$(kubectl get ingress kubelab-ingress -n kubelab -o jsonpath='{.status.loadBalancer.ingress[0].ip}')/api/orders

# 3. HPA configured?
kubectl get hpa -n kubelab

# 4. PDB in place?
kubectl get pdb -n kubelab

# 5. Network policies active?
kubectl get networkpolicy -n kubelab

# 6. CronJob scheduled?
kubectl get cronjob -n kubelab

# 7. Resource usage reasonable?
kubectl top pods -n kubelab

# 8. Clean up
cd infra/terraform && terraform destroy
```

## Congratulations

You've deployed a multi-service application to GKE covering:

- Infrastructure as Code (Terraform)
- Container builds (Docker multi-stage)
- All core Kubernetes resources
- GKE-specific features (Ingress, BackendConfig, Workload Identity)
- Scaling, resilience, and observability patterns
- CI/CD automation

Keep iterating — try swapping PostgreSQL for Cloud SQL, add Argo CD, or deploy to GKE Autopilot.
