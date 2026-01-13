#!/bin/bash
# Deploy UploadStream to Kubernetes

set -e

NAMESPACE="uploadstream"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo " Deploying UploadStream to Kubernetes..."
echo " Using manifests from: $SCRIPT_DIR"

# Create namespace
echo " Creating namespace: $NAMESPACE"
kubectl apply -f "$SCRIPT_DIR/01-namespace.yaml"

# Apply configurations
echo "  Applying secrets and configs..."
kubectl apply -f "$SCRIPT_DIR/02-secrets.yaml"
kubectl apply -f "$SCRIPT_DIR/03-configmap.yaml"
kubectl apply -f "$SCRIPT_DIR/06-migrations-configmap.yaml"

# Create storage
echo " Creating persistent volumes..."
kubectl apply -f "$SCRIPT_DIR/04-pvc.yaml"

# Deploy database
echo "Deploying PostgreSQL..."
kubectl apply -f "$SCRIPT_DIR/05-postgres.yaml"

# Wait for postgres to be ready
echo " Waiting for PostgreSQL to be ready..."
kubectl wait --for=condition=ready pod -l app=postgres -n $NAMESPACE --timeout=300s 2>/dev/null || true
sleep 10

# Deploy application
echo "Deploying UploadStream application..."
kubectl apply -f "$SCRIPT_DIR/07-uploadstream.yaml"

# Apply HPA and LB
echo "Configuring autoscaling and load balancing..."
kubectl apply -f "$SCRIPT_DIR/08-hpa.yaml"
kubectl apply -f "$SCRIPT_DIR/09-loadbalancer.yaml"

echo ""
echo " Deployment complete!"
echo ""
echo " Check deployment status:"
echo "   kubectl -n $NAMESPACE get all"
echo ""
echo " View logs:"
echo "   kubectl -n $NAMESPACE logs -l app=uploadstream -f"
echo ""
echo " Access the service:"
echo "   kubectl -n $NAMESPACE port-forward svc/uploadstream 50051:50051"
echo ""
