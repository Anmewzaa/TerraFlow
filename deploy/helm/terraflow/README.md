# TerraFlow Helm chart

Runs TerraFlow with its own Postgres (a StatefulSet with a volume) in one release.
For local and demo clusters; production uses a managed Postgres with Entra auth
(`--set postgres.enabled=false` and `env.TF_PG_*`).

```bash
# image from ghcr (public package, or create a pull secret and set image.pullSecret)
helm upgrade --install terraflow deploy/helm/terraflow -n terraflow --create-namespace

# image built locally (docker-desktop shares the Docker daemon, so no push is needed)
docker build -t terraflow:local .
helm upgrade --install terraflow deploy/helm/terraflow -n terraflow --create-namespace \
  --set image.repository=terraflow --set image.tag=local

bash scripts/portforward.sh        # http://localhost:8080, restarts when the pod changes
```

The database password is generated into the `<release>-postgres` Secret and kept across
upgrades and `helm uninstall` (the data volume is kept too). Delete the Secret and the
`data-<release>-postgres-0` PVC together to start from nothing.

The image is published by `.github/workflows/image.yml` on every `v*` tag.
