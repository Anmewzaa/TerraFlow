# 02 — Tech stack

Status: agreed (v2, updated 2026-11-09 for D-049: Vercel + Supabase demo target). Items marked *planned* are not in the code yet.

## Architecture

```mermaid
flowchart LR
    U[Browser<br/>React SPA] -- "HTTPS, session cookie<br/>(same origin)" --> T

    subgraph V[Vercel]
        T[TerraFlow<br/>Go BFF + API<br/>serves the built SPA]
    end

    T -- OIDC auth code flow + PKCE --> E[Entra ID]
    T -- "SQL over TLS<br/>(Supabase pooler)" --> P[(Supabase Postgres)]
    T -- "REST: read repo,<br/>create branch<br/>(1-hour installation token)" --> G[GitHub<br/>GitLab and Azure DevOps later]
```

Phase 8 (D-049) replaced the earlier Azure target (AKS, Azure PostgreSQL with Entra,
Key Vault, Workload Identity) with this demo profile. The Helm chart stays in the
repository but is no longer the target. A company profile (Azure DevOps repositories on a
host the company allows) is kept possible by the provider interface, generic OIDC and
plain Postgres.

There is no connection from Terraflow to Azure Resource Manager.

## Frontend

| Concern | Choice | Notes |
|---|---|---|
| Language / build | React + TypeScript + Vite | |
| Canvas | React Flow (xyflow) | |
| Auto-layout | ELK (`elkjs`) | Handles nested groups (RG ⊃ VNet ⊃ Subnet), which dagre does not handle well |
| Canvas state | Zustand | Also used internally by React Flow |
| Server state | TanStack Query | |
| Schema forms | Hand-written `SchemaForm` + custom widgets | Renders the admin-registered JSON Schema (D-015, D-019); widgets cover key/value, lists, locked fields and picking an existing resource. `react-jsonschema-form` is not used |
| Diff / code view | `react-syntax-highlighter` + a small line diff (`web/src/lib/diff.ts`) | HCL preview and the review screen. Monaco is not used |
| Styling / components | Tailwind CSS v4 (utilities only) + Harbor/Aurora design system | D-026, D-028. No Fluent UI or shadcn/ui; the question is closed |

## Backend

| Concern | Choice | Notes |
|---|---|---|
| Language | Go | |
| HTTP | `net/http` + `chi` | Compatible with the standard library |
| API contract | OpenAPI 3.1 (`api/openapi.yaml`) | Maintained by hand next to the handlers; the Go server and the TS types (`web/src/types/`) are **not** generated yet (*planned*: add generation and a drift check) |
| HCL | `hashicorp/hcl/v2` + `hclwrite` | Keeps comments, formatting and ordering, so PR diffs stay small |
| Expression evaluation | `go-cty` | Evaluates `var`, `locals` and `.tfvars` for the inventory |
| DB access | `pgx` with hand-written SQL in `server/internal/store/` | No ORM and no `sqlc` so far (*planned* if the query count grows) |
| Migrations | `goose` (001-012, additive, each with a working `Down`) | Embedded SQL. In Phase 8 they run as a separate command (`terraflow migrate`) at deploy, not at every start (D-049) |
| Canvas storage | JSONB | |
| Git provider | `internal/gitprovider` (*planned*, Phase 8): GitHub App client first, `local` (the current `localrepo` with `git worktree`, D-027) for development and tests; GitLab and Azure DevOps later. Hand-written REST clients | `internal/ado` stays empty until the Azure DevOps provider. Same one-commit-on-a-new-branch contract as `BranchWriter` |
| Logging | `slog` (JSON) | |

## Security

| Concern | Choice |
|---|---|
| User auth | Entra ID with the **BFF pattern** (the local build can also sign in as a made-up user, `TF_AUTH_DISABLED` + `TF_DEV_LOGIN`, D-043; never active with auth on): the backend runs the OIDC flow and keeps the session in an httpOnly, Secure, SameSite cookie. Tokens never reach the browser |
| App credential | The Entra app registration uses a client secret held in the host environment (`TF_ENTRA_CLIENT_SECRET`); a federated credential from the host's OIDC is optional (D-049) |
| Git credentials | GitHub App: only the installation id is stored; each call exchanges a signed JWT for a one-hour token limited to one repository (`contents: write`, `metadata: read`); tokens are never stored; the App private key lives only in the environment. Later providers: encrypted refresh token or service principal (D-049) |
| PostgreSQL auth | Password in the host environment, TLS required; Supabase pooler in transaction mode; row level security on every table with no policy (migration 014). Entra passwordless is optional for a company profile |
| Secrets | Only in the host's environment variables (Vercel): database URL, Entra client secret, GitHub App private key, admin list. The session cookie is a random id and Postgres stores its SHA-256 (D-047) |
| Branch permissions | The credential can create a branch on the chosen repository only. Branch protection on the default branch is required in the target repository, because a GitHub App cannot be limited to branch names. Branch policy is never bypassed |
| Azure permissions | **None.** No ARM or subscription access, no Azure credential at all |
| Login hardening (D-047) | PKCE (S256) and a nonce check on every login; session ids are stored as SHA-256; CORS allows only `TF_ALLOWED_ORIGINS` (none by default); writes a browser marks cross-site are refused; security headers on every response; sign-in and callback are rate limited; the server will not start with `TF_AUTH_DISABLED=true` and Entra settings together |
| Group overage | The token should contain only groups assigned to the application. The backend detects the overage claim and fails with a clear error. It does not call Microsoft Graph |

**Why one identity:** a single pod would hold every identity anyway, so splitting
identities would not reduce the blast radius. Least privilege is applied on each
target instead. Real separation would need a separate branch-writer service, which
can be revisited if security review requires it.

## Deploy

| Concern | Choice |
|---|---|
| Packaging | The Go server and the built SPA on Vercel (Go framework preset, listens on `PORT`), one origin. The Docker image remains for local clusters |
| Runtime limits | No `git` binary, no persistent disk, no shared memory between instances: the provider API replaces `localrepo`, state lives in Postgres, migrations are a separate command (D-049) |
| Orchestration | Vercel. The Helm chart is kept but is not the target |
| Build | Dependencies can be built and packaged outside the air-gapped network and brought in |
| CI pipeline | GitHub Actions: Go and web checks with a Postgres service (D-013 withdrawn, D-049) |

## Testing

| Type | What it checks | When |
|---|---|---|
| Unit | Go (`testing`), frontend (Vitest) | From Phase 1 |
| Postgres | Store and API tests run against a throwaway database when `TF_TEST_PG_DSN` is set, and are **skipped** otherwise | Run them before every PR |
| **Golden file** | Canvas JSON → generated `.tf`, compared byte-for-byte with expected files | From Phase 4 (the most important test) |
| **Round-trip** | Parse → modify → write. Untouched parts, including comments, stay byte-identical | From Phase 1 |
| Integration | Real Postgres in a container. the git provider mocked with `httptest` | From Phase 2 |
| `terraform validate` | Generated output is valid Terraform (needs a provider mirror) | Later |
| E2E | Full browser flow (Playwright) | Later |
