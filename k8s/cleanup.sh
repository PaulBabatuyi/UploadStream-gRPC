#!/bin/bash
# Cleanup UploadStream deployment from Kubernetes

set -e

NAMESPACE="uploadstream"

echo "🗑️  Removing UploadStream from Kubernetes..."

# Delete in reverse order
kubectl delete namespace $NAMESPACE --ignore-not-found

echo "✅ Cleanup complete!"
