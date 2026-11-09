# 10 — Security: threat model, secret handling, identity scope

Status: Phase 7 review of the local build (2026-11-09). It describes the code as it
is, says which test covers each control, and lists what is still open for Phase 8.
Decisions behind it: D-008..D-011, D-032, D-037, D-046..D-048.

## What is protected

| Asset | Why it matters |
|---|---|
| The session cookie | Whoever holds it acts as that user, including an admin who can approve |
| The module catalog and raw resource templates | They decide what code can be generated |
| The Terraform that Approve writes to a branch | It is what DevOps reviews and, after a PR, applies |
| Project visibility and audit log | Who may see which project, and the record of who did what |
| The database and its credentials | Holds all of the above |

TerraFlow holds **no Azure credentials** and calls no Azure API. Today it makes only
three kinds of outbound or local calls: OIDC to the identity provider, Postgres, and
the `git` CLI on a local repository (Phase 8 replaces the last with the ADO REST API).

## Trust boundaries

1. Browser to server (HTTPS, session cookie).
2. Server to identity provider (Entra OIDC).
3. Server to Postgres.
4. Server to the repository (a local folder now, ADO in Phase 8).
5. Generated `.tf` to DevOps review in ADO (the human gate; TerraFlow stops at the branch).

## Threats and controls

| # | Threat | Control | Test |
|---|---|---|---|
| 1 | Forged or replayed login response | `state` cookie checked; PKCE (S256) code exchange; ID token nonce compared with the cookie | `auth/security_test.go`: `TestCallback_RejectsBadState`, `TestLogin_UsesPKCE`, `TestNonceMatches` |
| 2 | Stolen session id from a database backup or SQL read | Only the SHA-256 of the id is stored; expired sessions purged | `store/session_queries_test.go` |
| 3 | Cross-site request forgery | `SameSite=Lax` cookie plus refusal of writes marked cross-site or with a foreign `Origin` | `api/hardening_test.go`: `TestSameOriginWrites`, `TestRouterAppliesTheHardening` |
| 4 | Another website calling the API from a browser | No wildcard CORS; only `TF_ALLOWED_ORIGINS` | `TestCORS` |
| 5 | Clickjacking, sniffing | `X-Frame-Options`, `nosniff`, CSP, HSTS | `TestSecurityHeaders` |
| 6 | Password guessing or flooding sign-in | Rate limit on `/auth/login` and `/auth/callback`. Entra does the credential check | `TestRateLimit` |
| 7 | A developer using admin routes | Admin routes need the `devops` role (403) | `TestEveryAdminRouteRefusesDevelopers` (walks every route) |
| 8 | Seeing or touching a project one is not a member of | Project guard answers 404, identical to a missing project | `TestEveryProjectRouteHidesProjectsFromNonMembers`, `TestNonMemberCannotTellRealFromMissingProject` |
| 9 | Reading or changing another project's changeset by id | `ChangesetInProject` | `TestChangesetsAreScopedToTheirProject` |
| 10 | Reading another member's draft | Drafts and saved versions belong to their author (D-044) | `TestDraftsBelongToTheirAuthor` |
| 11 | Approving one's own change | Only `devops`; optional separation of duties | `TestMemberCannotReviewTheirOwnChangeset` |
| 12 | User text changing the generated Terraform (injection) | Strings escape `${` and `%{`; names, files, folders, resource types and env are validated; references only through `$expr` | `hcl/injection_test.go` |
| 13 | Path traversal through `env`, folder or file names | `env` must match `^[A-Za-z0-9][A-Za-z0-9_-]{0,31}$`; `localrepo` refuses `..`, absolute and drive paths | `TestValidEnv`, `TestOddEnvIsRefused`, `localrepo` tests |
| 14 | Oversized or malformed request bodies | `MaxBytesReader` on the admin and review write handlers; bad ids answer 404/400, not 500 | `TestOversizedBodiesAreRefused`, `TestMalformedBodiesAreClientErrors`, `TestMalformedIDsAreNotFoundNotServerErrors` |
| 15 | Dev sign-in left on in production | Server refuses to start with `TF_AUTH_DISABLED=true` and any `TF_ENTRA_*` set; dev routes absent otherwise | `auth/mode_test.go`, `TestDevLoginRoutesAbsentWithoutDevLogin` |
| 16 | A bug that makes DevOps hand-write `.tf` or applies unreviewed code | The app stops at a branch; the PR and pipeline in ADO are the gate (hard rules 1 and 7) | design, not a test |
| 17 | Dependency vulnerabilities | `npm audit --omit=dev` is clean (vite 6.4.6, react-syntax-highlighter 16.1.2) | run it before each release |
| 18 | Running as root in the container | Image runs as UID 65532; the chart sets `runAsNonRoot`, drops all capabilities | **Not tested on a cluster** (no Docker or Kubernetes in the build environment) |

## Accepted risks and open items

| Item | Status |
|---|---|
| Roles are read once at sign-in; a user removed from an Entra group keeps the role until the session ends (4 hours). No Graph calls (D-010) | Accepted; shorten `sessionDuration` if too long |
| Rate limit is per pod and trusts `X-Forwarded-For` from the ingress | Accepted for one pod; use the ingress limiter when scaled out |
| `GET /auth/logout` changes state on a GET (logout CSRF only signs the user out) | Accepted, low |
| `/api/graph?dir=` reads any directory the process can read | Only registered with `TF_NO_DB=true` (dev); never in database mode |
| Graph errors return the parser message, which can include a server path | Accepted for the local build; review in Phase 8 when paths are remote |
| `go` vulnerability scan (`govulncheck`) could not run: the build environment blocks `vuln.go.dev` | **Open**: run it where the database is reachable |
| `git config --system safe.directory '*'` in the image | Needed because the workspace volume is owned by another user; revisit when `internal/ado` replaces `git` |
| No CI job runs the Postgres tests; without `TF_TEST_PG_DSN` they are skipped (D-013 keeps CI out of scope) | Open decision |
| Browser tests need a running stack (`web/e2e/README.md`) | Run before release |

## Secret-handling checklist

| Item | Today | Needed before production |
|---|---|---|
| Entra client secret | `TF_ENTRA_CLIENT_SECRET` read from the environment | Federated credential on Workload Identity, no secret (Phase 8) |
| Session cookie key | Sessions are random ids in Postgres; no signing key is used | Nothing; Key Vault holds only what the app needs later |
| Database password | `TF_PG_PASSWORD`; default `localdev` in `main.go`; the Helm chart generates a random one for the bundled Postgres | Entra passwordless auth; remove the default (Phase 8) |
| DB role | One role runs both migrations and the app | A separate migration identity for the pre-deploy Job and a least-privilege runtime role |
| Logs | Auth logs carry errors only; no tokens, cookies or codes | Keep it so; a log test is not in place |
| Fixtures and samples | Synthetic | Keep synthetic |
| Image and chart | No secrets in the image; the Postgres password is a Kubernetes Secret | Key Vault CSI for anything new |

## Identity-scope audit

What each identity can do, today and in Phase 8.

| Identity | Today | Phase 8 target (least privilege) |
|---|---|---|
| The browser user | A session with a role (`devops` or `developer`) from Entra group claims; project access from `project_members` | Same |
| The TerraFlow process, Azure | Nothing. No ARM or Azure SDK call exists in the code | Workload Identity with access to its own PostgreSQL and Key Vault only; **no** ARM or subscription role |
| The TerraFlow process, ADO | Nothing (`internal/ado` is empty) | Contribute and Create branch on the target repositories only. Prefer branch security so Contribute is allowed only on `refs/heads/terraflow/*`, with `main` protected by branch policy, so a bug cannot push to a protected branch |
| The TerraFlow process, database | One full-rights role | Runtime role limited to the application tables; separate migration role |
| The TerraFlow process, filesystem | Writes `/workspace`; runs `git` with arguments built from validated names | Not applicable once ADO replaces the local repository |
| Entra app registration | Delegated scopes `openid profile email`; group claims for roles | Same; keep the token limited to groups assigned to the application (D-010) |

Result: in the current build the process cannot affect Azure at all. The scope
that needs the most care is the ADO permission in Phase 8, because "Contribute" on a
whole repository is wider than "create a new branch".
