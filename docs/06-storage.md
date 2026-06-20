# Module 06 — Storage

Persistent data in Kubernetes: volumes, claims, and StatefulSets.

## The Problem

Pod filesystems are **ephemeral**. When a pod dies, its data is gone. Databases need persistent storage.

## Key Objects

| Object | Purpose |
|--------|---------|
| **PersistentVolume (PV)** | Piece of storage in the cluster |
| **PersistentVolumeClaim (PVC)** | Request for storage (pod asks for 5Gi) |
| **StorageClass** | Dynamic provisioning template |
| **Volume** | Mount storage into a pod |

On GKE, you usually only create PVCs — GKE dynamically provisions Google Persistent Disks.

## Our PostgreSQL StatefulSet

```yaml
# k8s/base/postgres/statefulset.yaml
volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: ["ReadWriteOnce"]
      storageClassName: standard-rwo    # GKE SSD, ReadWriteOnce
      resources:
        requests:
          storage: 5Gi
```

Each StatefulSet pod gets its own PVC: `data-postgres-0`

```bash
kubectl get pvc -n kubelab
kubectl get pv
```

## StatefulSet vs Deployment

| | Deployment | StatefulSet |
|---|-----------|-------------|
| Pod names | Random hash | Ordered: `postgres-0` |
| Storage | Shared or none | Dedicated per pod |
| Network | Random | Stable DNS per pod |
| Scale | Any order | Ordered 0, 1, 2... |

Use StatefulSet for: databases, Kafka, etcd, anything needing stable identity.

## Headless Service

PostgreSQL uses a headless Service (`clusterIP: None`) so DNS returns pod IPs directly:

```
postgres-0.postgres.kubelab.svc.cluster.local → 10.x.x.x
```

## GKE Storage Classes

| Class | Disk type | Use |
|-------|-----------|-----|
| `standard-rwo` | Standard PD | General purpose (our default) |
| `premium-rwo` | SSD | High IOPS databases |
| `standard-rwx` | Filestore | ReadWriteMany (shared) |

```bash
kubectl get storageclass
```

## Backup Exercise (Manual)

```bash
# Port-forward postgres
kubectl port-forward postgres-0 -n kubelab 5432:5432

# Dump database (in another terminal)
PGPASSWORD=kubelab pg_dump -h localhost -U kubelab kubelab > backup.sql
```

For production: use Cloud SQL or Velero for automated backups.

## Exercise

1. `kubectl describe pvc -n kubelab` — note the bound PV
2. `kubectl exec -it postgres-0 -n kubelab -- psql -U kubelab -c '\dt'`
3. Create an order via the UI, then verify in postgres directly
4. **Careful:** delete the pod `kubectl delete pod postgres-0 -n kubelab` — data survives because of PVC

## Next

→ [Module 07: Secrets & Config](07-secrets-config.md)
