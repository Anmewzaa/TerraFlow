#!/usr/bin/env bash
# Install or upgrade TerraFlow on the current Kubernetes context, in one of the two versions:
#
#   bash scripts/helm-install.sh local            # the app + its own Postgres, no GitHub (made-up users)
#   bash scripts/helm-install.sh github [helm args...]
#                                                 # the app + Postgres + sign-in with GitHub and GitHub projects
#
# Environment (all optional):
#   NAMESPACE   default terraflow
#   RELEASE     default terraflow
#   IMAGE       default the chart's (ghcr.io/anmewzaa/terraflow); IMAGE=terraflow:local uses a local build
#
# The github version needs a Secret first; see deploy/helm/terraflow/README.md. Extra arguments go to helm, for
# example:  bash scripts/helm-install.sh github --set github.appId=123456 --set github.appSlug=my-app ...
set -euo pipefail

version="${1:-}"
[ -n "$version" ] || { sed -n '2,13p' "$0" | sed 's/^# \{0,1\}//'; exit 2; }
shift || true

ns="${NAMESPACE:-terraflow}"
release="${RELEASE:-terraflow}"
chart="$(cd "$(dirname "$0")/.." && pwd)/deploy/helm/terraflow"

image_args=()
if [ -n "${IMAGE:-}" ]; then
  image_args=(--set "image.repository=${IMAGE%%:*}" --set "image.tag=${IMAGE##*:}")
fi

case "$version" in
  local)
    values="$chart/values-local.yaml"
    ;;
  github)
    values="$chart/values-github.yaml"
    secret="terraflow-github"
    if ! kubectl -n "$ns" get secret "$secret" >/dev/null 2>&1; then
      echo "The Secret '$secret' is missing in namespace '$ns'. Create it first (the GitHub App's private key and client secret):" >&2
      echo "  kubectl create namespace $ns" >&2
      echo "  kubectl -n $ns create secret generic $secret --from-file=private-key.pem=./github-app.pem --from-literal=client-secret=\"\$GITHUB_CLIENT_SECRET\"" >&2
      exit 1
    fi
    ;;
  *)
    echo "Unknown version '$version': use 'local' or 'github'." >&2
    exit 2
    ;;
esac

helm upgrade --install "$release" "$chart" -n "$ns" --create-namespace -f "$values" "${image_args[@]}" "$@" --wait --timeout 5m

echo
echo "Installed '$release' in '$ns' ($version version)."
if [ "$version" = local ]; then
  echo "Open it:  kubectl -n $ns port-forward svc/$release 8080:8080   then http://localhost:8080"
else
  echo "Open it at the host you set under ingress (or the redirect URI you gave the GitHub App)."
fi
