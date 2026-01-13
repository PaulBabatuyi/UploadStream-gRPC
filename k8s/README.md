# Kubernetes Deployment Guide

## Prerequisites

1. **Kubernetes cluster running** (Docker Desktop, Minikube, kind, cloud provider)
   ```bash
   # Check cluster
   kubectl cluster-info
   kubectl get nodes
   ```

2. **Docker image built and available**
   ```bash
   # Build the image
   docker build -t uploadstream:latest .
   
   # For Docker Desktop or Minikube, the image is available locally
   # For remote clusters, push to registry first:
   # docker tag uploadstream:latest <registry>/uploadstream:latest
   # docker push <registry>/uploadstream:latest
   # Then update k8s/07-uploadstream.yaml imagePullPolicy: IfNotPresent → Always
   ```

## Deploy to Kubernetes

### Option 1: Using the deploy script (recommended)
```bash
cd k8s
chmod +x deploy.sh cleanup.sh
./deploy.sh
```

### Option 2: Manual deployment
```bash
kubectl apply -f k8s/01-namespace.yaml
kubectl apply -f k8s/02-secrets.yaml
kubectl apply -f k8s/03-configmap.yaml
kubectl apply -f k8s/06-migrations-configmap.yaml
kubectl apply -f k8s/04-pvc.yaml
kubectl apply -f k8s/05-postgres.yaml

# Wait for postgres
kubectl wait --for=condition=ready pod -l app=postgres -n uploadstream --timeout=300s

kubectl apply -f k8s/07-uploadstream.yaml
kubectl apply -f k8s/08-hpa.yaml
kubectl apply -f k8s/09-loadbalancer.yaml
```

## Verify Deployment

```bash
# Check all resources
kubectl -n uploadstream get all

# Check pods
kubectl -n uploadstream get pods -w

# Check services
kubectl -n uploadstream get svc

# View logs
kubectl -n uploadstream logs -l app=uploadstream -f

# Specific pod logs
kubectl -n uploadstream logs <pod-name> -f
```

## Access the Service

### Port-forward for local testing
```bash
# gRPC service
kubectl -n uploadstream port-forward svc/uploadstream 50051:50051

# Metrics
kubectl -n uploadstream port-forward svc/uploadstream 9090:9090

# Then test with client:
grpcurl -plaintext localhost:50051 list
```

### External access (LoadBalancer)
```bash
# Get external IP
kubectl -n uploadstream get svc uploadstream-external

# For Docker Desktop or Minikube, use localhost:30051
# For cloud (AWS/GCP/Azure), wait for LoadBalancer external IP to be assigned
```

## Scaling

### Manual scaling
```bash
kubectl -n uploadstream scale deployment uploadstream --replicas=3
```

### Check HPA status
```bash
kubectl -n uploadstream get hpa
kubectl -n uploadstream describe hpa uploadstream-hpa
```

## Updating

### Update image (rebuild and redeploy)
```bash
docker build -t uploadstream:latest .
kubectl -n uploadstream rollout restart deployment uploadstream
kubectl -n uploadstream rollout status deployment uploadstream
```

### Update database secrets
```bash
kubectl -n uploadstream delete secret uploadstream-db-secret
kubectl -n uploadstream create secret generic uploadstream-db-secret \
  --from-literal=DB_PASSWORD=newpassword \
  --from-literal=DATABASE_URL="postgres://uploader:newpassword@postgres:5432/uploadstream?sslmode=disable"
kubectl -n uploadstream rollout restart deployment uploadstream
```

## Cleanup

```bash
./cleanup.sh
# Or manually:
kubectl delete namespace uploadstream
```

## Troubleshooting

### Pods not starting
```bash
kubectl -n uploadstream describe pod <pod-name>
kubectl -n uploadstream logs <pod-name>
```

### Database connection issues
```bash
# Check postgres is ready
kubectl -n uploadstream get pod -l app=postgres
kubectl -n uploadstream logs postgres-0

# Test database connection from a pod
kubectl -n uploadstream exec -it uploadstream-<hash> -- psql \
  -h postgres -U uploader -d uploadstream -c "SELECT version();"
```

### Storage issues
```bash
kubectl -n uploadstream get pvc
kubectl -n uploadstream get pv
```

## Production Considerations

- Use managed PostgreSQL (AWS RDS, Cloud SQL, Azure Database)
- Push images to a container registry (Docker Hub, ECR, GCR, ACR)
- Use secrets management (Vault, AWS Secrets Manager, etc.)
- Configure RBAC properly
- Add network policies
- Use persistent volumes from cloud provider
- Enable pod security policies
- Add resource quotas and limits
- Configure backup/restore procedures
