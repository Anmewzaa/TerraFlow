#!/usr/bin/env bash
# Keeps `kubectl port-forward` alive: it dies whenever the pod behind the service is replaced.
# Usage: bash scripts/portforward.sh [local-port]   (stop with Ctrl+C)
PORT="${1:-8080}"
while true; do
  kubectl -n terraflow port-forward svc/terraflow "${PORT}:8080" || true
  echo "port-forward ended, restarting in 2s..." >&2
  sleep 2
done
