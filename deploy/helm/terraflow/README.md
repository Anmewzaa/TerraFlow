# TerraFlow Helm chart

One chart, two versions. Both run the app **and its own Postgres** (a StatefulSet with a volume) in one release.

| | Local version | GitHub version |
|---|---|---|
| Values file | `values-local.yaml` | `values-github.yaml` |
| Sign-in | pick a made-up user (admin or developer) | sign in with GitHub; a new person waits for an admin |
| Projects | folders in the cluster (samples to start from) | your GitHub repositories (a branch per approved change) |
| Needs | nothing but a cluster | a GitHub App, a Secret, and https |
| For | a laptop, a demo, trying it out | a team |

TerraFlow never talks to Azure in either version, and stops at the branch.

```bash
bash scripts/helm-install.sh local        # or: bash scripts/helm-install.sh github
```

The script is a thin wrapper over `helm upgrade --install ... -f values-<version>.yaml`; the commands below do the same by hand.

## Local version

```bash
# image from ghcr (public package, or create a pull secret and set image.pullSecret)
helm upgrade --install terraflow deploy/helm/terraflow -n terraflow --create-namespace \
  -f deploy/helm/terraflow/values-local.yaml

# image built on this machine (docker-desktop shares the Docker daemon, so no push is needed)
docker build -t terraflow:local .
helm upgrade --install terraflow deploy/helm/terraflow -n terraflow --create-namespace \
  -f deploy/helm/terraflow/values-local.yaml --set image.repository=terraflow --set image.tag=local

bash scripts/portforward.sh        # http://localhost:8080, restarts when the pod changes
```

Open the page, press **Get Started**, and pick a user. This version is for plain http on one machine: it turns off
real sign-in, so do not expose it to other people.

## GitHub version

1. **Create a GitHub App** (Settings > Developer settings > GitHub Apps > New). Give it:
   - *Callback URLs* (both): `https://<host>/api/github/callback` (connecting repositories) and
     `https://<host>/auth/github/callback` (sign-in). Tick "Request user authorization (OAuth) during installation".
   - *Repository permissions*: Contents: read and write, Metadata: read. Nothing more: TerraFlow makes a branch and never opens a pull request.
   - Generate a **private key** (a `.pem` file) and a **client secret**. Note the App id, its slug (the name in its URL) and the client id.
2. **Create the Secret** next to the release (the key and the secret never go in a values file):

   ```bash
   kubectl create namespace terraflow
   kubectl -n terraflow create secret generic terraflow-github \
     --from-file=private-key.pem=./github-app.pem \
     --from-literal=client-secret="$GITHUB_CLIENT_SECRET"
   ```

3. **Install**, giving the App's public details (or edit `values-github.yaml`):

   ```bash
   helm upgrade --install terraflow deploy/helm/terraflow -n terraflow \
     -f deploy/helm/terraflow/values-github.yaml \
     --set github.appId=123456 --set github.appSlug=my-terraflow --set github.clientId=Iv1.xxxxxxxx \
     --set github.redirectUri=https://terraflow.example.com/api/github/callback \
     --set github.loginRedirectUri=https://terraflow.example.com/auth/github/callback \
     --set github.adminUsers=github:12345 \
     --set ingress.enabled=true --set ingress.host=terraflow.example.com --set ingress.tlsSecret=terraflow-tls
   ```

   `github.adminUsers` is who administers it (`github:<numeric id>`; `https://api.github.com/users/<login>` shows the id).
   `github.allowedOwners` limits which accounts may install the App. For a quick try over http on localhost, add
   `--set github.insecureCookies=true` and port-forward; real use is https.

Install fails early, with a message, when a required value or the Secret is missing.

## What the chart keeps

- The database password is generated into the `<release>-postgres` Secret and kept across upgrades and `helm uninstall`
  (the data volume is kept too). Delete the Secret and the `data-<release>-postgres-0` PVC together to start from nothing.
- The key that signs session cookies is generated into `<release>-app` and kept the same way, so an upgrade does not sign everyone out.
  Set `sessionKey` to choose your own.
- `/workspace` is a volume (`persistence.enabled`), so it survives restarts and upgrades.

## Production notes

The bundled Postgres is for a team that wants one command. For a managed Postgres set `postgres.enabled=false` and
`env.TF_PG_*` (or Entra passwordless auth). Hosting is not decided yet (D-049), so nothing here sets up DNS, certificates or backups.

The image is published by `.github/workflows/image.yml` on every `v*` tag (the chart's `appVersion` and `image.tag` are 1.0.0).
