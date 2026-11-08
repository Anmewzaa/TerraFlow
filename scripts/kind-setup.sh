#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-terraflow}"
IMAGE_TAG="${IMAGE_TAG:-local}"
NAMESPACE="${NAMESPACE:-default}"

echo "==> Building Docker image..."
docker build -t "terraflow:${IMAGE_TAG}" .

echo "==> Creating kind cluster (if needed)..."
if ! kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  kind create cluster --name "${CLUSTER_NAME}"
fi

echo "==> Loading image into kind..."
kind load docker-image "terraflow:${IMAGE_TAG}" --name "${CLUSTER_NAME}"

echo "==> Installing Helm chart..."
helm upgrade --install terraflow deploy/helm/terraflow \
  --namespace "${NAMESPACE}" \
  --set image.repository=terraflow \
  --set image.tag="${IMAGE_TAG}" \
  --set env.TF_AUTH_DISABLED=true

echo "==> Waiting for rollout..."
kubectl rollout status deployment/terraflow --namespace "${NAMESPACE}" --timeout=120s

echo ""
echo "Done! Run:"
echo "  kubectl port-forward svc/terraflow 8080:8080"
echo "Then open http://localhost:8080"
