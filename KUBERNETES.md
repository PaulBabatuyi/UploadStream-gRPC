# UploadStream-gRPC: DevOps Quick Start

##  Architecture Overview

```
Docker Desktop / Minikube
    ↓
Kubernetes Cluster (uploadstream namespace)
    ├── PostgreSQL StatefulSet (1 pod)
    ├── UploadStream Deployment (2-5 pods, auto-scaling)
    ├── Services (ClusterIP + LoadBalancer)
    └── Persistent Volumes (postgres + file storage)
```

##  Quick Deployment

### 1. Prepare Docker Image
```bash
# Build locally
docker build -t uploadstream:latest .

# Verify image
docker images | grep uploadstream
```

### 2. Deploy to Kubernetes
```bash
cd k8s
chmod +x deploy.sh
./deploy.sh
```

### 3. Verify Deployment
```bash
kubectl -n uploadstream get all
kubectl -n uploadstream get pods -w  # Watch pod creation
```

### 4. Test Service
```bash
# Port-forward to test
kubectl -n uploadstream port-forward svc/uploadstream 50051:50051 &

# In another terminal, run client
go run cmd/client/main.go
```

## Kubernetes Manifests Included

| File | Purpose |
|------|---------|
| `01-namespace.yaml` | Create uploadstream namespace |
| `02-secrets.yaml` | DB credentials (encrypted at rest) |
| `03-configmap.yaml` | App configuration |
| `04-pvc.yaml` | Persistent volume claims |
| `05-postgres.yaml` | PostgreSQL StatefulSet |
| `06-migrations-configmap.yaml` | Database migrations |
| `07-uploadstream.yaml` | App Deployment + Service |
| `08-hpa.yaml` | Auto-scaling configuration |
| `09-loadbalancer.yaml` | External LoadBalancer service |

##  Common Operations

### View logs
```bash
kubectl -n uploadstream logs -l app=uploadstream -f
kubectl -n uploadstream logs postgres-0 -f
```

### Scale manually
```bash
kubectl -n uploadstream scale deployment uploadstream --replicas=4
```

### Update deployment
```bash
# After code changes:
docker build -t uploadstream:latest .
kubectl -n uploadstream rollout restart deployment uploadstream
kubectl -n uploadstream rollout status deployment uploadstream
```

### Delete everything
```bash
./cleanup.sh
# Or: kubectl delete namespace uploadstream
```

##  What's Configured

 **Auto-scaling** - 2-5 replicas based on CPU/memory  
**Health checks** - Liveness & readiness probes  
 **Resource limits** - CPU and memory quotas  
 **Persistent storage** - 10GB postgres + 50GB file storage  
 **Database migrations** - Automatic on startup  
 **Service discovery** - Internal and external access  
 **Graceful shutdown** - Pod disruption budgets (optional)  

##  Before Production

1. **Change DB password** in `02-secrets.yaml`
2. **Use external database** (AWS RDS, Cloud SQL, etc.)
3. **Push image to registry** (Docker Hub, ECR, GCR, etc.)
4. **Enable TLS** for gRPC (add Ingress with cert)
5. **Configure RBAC** and network policies
6. **Set resource quotas** per namespace
7. **Implement backup** strategy for persistent volumes
8. **Use secrets management** (Vault, AWS Secrets Manager)

##  Further Reading

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [StatefulSet patterns](https://kubernetes.io/docs/tutorials/stateful-application/basic-stateful-set/)
- [gRPC on Kubernetes](https://kubernetes.io/blog/2018/07/grpc-loadbalancing-on-kubernetes-without-tears/)
