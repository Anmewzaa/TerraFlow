# 10 — Security: threat model, secret handling, identity scope

Status: Phase 7 review of the local build (2026-11-09), extended with the Phase 8
design (D-049). It describes the code as it is, says which test covers each control,
and lists what is open. Decisions behind it: D-008..D-011, D-032, D-037, D-046..D-050.

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
the `git` CLI on a local repository (Phase 8 replaces the last with the GitHub API).

## Trust boundaries

1. Browser to server (HTTPS, session cookie).
2. Server to identity provider (Entra OIDC).
3. Server to Postgres.
4. Server to the repository (a local folder now, GitHub in Phase 8).
5. Generated `.tf` to review in the git provider (the human gate; TerraFlow stops at the branch).

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
| Entra client secret | `TF_ENTRA_CLIENT_SECRET` read from the environment | Held only in the host's environment; rotate on a schedule |
| GitHub App private key and client secret | Planned (`TF_GITHUB_APP_*`); the key stays in a file outside the repository or in the host's secret store | One key can mint tokens for all installations (G1): environment only, never logged, rotation documented |
| Session cookie key | Sessions are random ids in Postgres; no signing key is used | Nothing |
| Database password | `TF_PG_PASSWORD`; default `localdev` in `main.go`; the Helm chart generates a random one for the bundled Postgres | One URL setting with TLS; remove the default (Phase 8) |
| DB role | One role runs both migrations and the app | A separate migration role and a least-privilege runtime role |
| Local settings file | `.env.local` / `.env` are read by the server for development (`server/internal/envfile`); both are git-ignored, only `.env.example` (placeholders) is committed; the loader logs variable names, never values, and a bad line is reported by number only | Key files (`*.pem`) stay outside the repository; a deployment uses real environment variables, which win over the file |
| Logs | Auth logs carry errors only; no tokens, cookies or codes | Keep it so; a log test is not in place |
| Fixtures and samples | Synthetic | Keep synthetic |
| Image and chart | No secrets in the image; the Postgres password is a Kubernetes Secret | Key Vault CSI for anything new |

## Identity-scope audit

What each identity can do, today and after Phase 8 (D-049, D-050).

| Identity | Today (local build) | Phase 8 |
|---|---|---|
| The browser user | A session with a role (`devops` or `developer`) from Entra group claims; project access from `project_members` | Signed in with GitHub or Entra; no global role unless in `TF_ADMIN_USERS`; a role per project (owner, co-owner, contributor) |
| TerraFlow, Azure | Nothing. No ARM or Azure SDK call exists in the code | Nothing, and no Azure credential at all |
| TerraFlow, GitHub | Nothing | A GitHub App: `contents: write` and `metadata: read` on the repositories an owner installed it on; each call uses a one-hour token limited to one repository. A user token is used during one request and discarded |
| TerraFlow, database | One full-rights role | A runtime role limited to the application tables and a separate migration role |
| TerraFlow, filesystem | Writes `/workspace`; runs `git` with validated arguments | None: no `git` binary, no disk (the `local` provider is development only) |
| Entra app registration | Delegated scopes `openid profile email`; group claims for roles | Same; the client secret is held in the environment |

The scope that needs the most care is the GitHub App. It cannot be limited to branch names, so
the target repository must protect its default branch (branch protection or a ruleset), or a bug
could push to it.

## GitHub connector design (Phase 8)

| # | Threat | Control (planned) |
|---|---|---|
| G1 | Theft of the App private key lets anyone mint tokens for every installation, now across all users' repositories | Held only in the environment; never logged or returned; GitHub allows several keys, so rotate by adding a new key then removing the old; each token is limited to one repository; a test that no response or log line contains key material. **The largest single risk of the phase** |
| G2 | A leaked token is reused | Tokens last one hour, are limited to one repository, and are never stored; a 401 mints a new one once |
| G3 | A forged `installation_id` on the return from GitHub links someone else's installation | Check `state` (CSRF), then verify with GitHub, using the user's own token, that this user can use that installation before saving it |
| G4 | A user connects a repository they cannot access (the App as a confused deputy) | The list contains only repositories that the installation can see and that this user can access (checked with the user's token at connect time); the server rechecks the chosen repository and folder on save |
| G5 | A branch pushed to a protected branch | Never forced; refused if the branch exists; the name is built by the server (`terraflow/<project>/<env>-<id8>`); the repository must protect its default branch (documented; the connection check warns if it does not) |
| G6 | A stale plan overwrites newer work | The plan records the base commit; Approve returns 409 if it moved |
| G7 | API rate limits turn into outages | Reads cached by commit SHA; one retry only; a clear error to the user |
| G8 | Server-side request forgery through a provider URL | GitHub.com only; when another provider is added: https only, no private or link-local addresses, no redirects to them |
| G9 | Connect or disconnect without a trace | Audited with who, which installation and which repository |
| G10 | The user's GitHub token is kept or leaked | Used during the request only and never stored or logged; a test checks that no response, log line or table holds it |

**Status of the GitHub controls (2026-11-09).** Implemented and tested against a fake GitHub that
enforces the real protocol (not yet against github.com): G1 (the key is never printed, logged or
returned; it is loaded from the environment by `githubFromEnv`; rotation guidance is written), G2
(one-hour tokens, one repository, two permissions, made per operation, one retry on 401), G3 (the
`state` is a random value in a short HttpOnly cookie and is checked in constant time; the installations
come from GitHub with the user's own token, never from a parameter of the return URL), G4 (a repository
can only be connected through a *grant*: a row that exists only because GitHub said, minutes ago and
with this user's token, that this user can reach it through this installation; grants are per user,
expire after ten minutes and are used up when the project is created; the server rechecks the folder
and the Terraform on save), G5 (never forced, refused if the name is taken, the name is built by the
server), G6 in a different form (the branch is made from the planned commit, D-051), G7 (rate-limit answers are recognised, one retry for tokens only, snapshots cached by tree SHA and blobs by blob SHA), G9 (`github.authorize`,
`project.create`, `github.disconnect` and `github.recheck` are audited with who, account and
repository), G10 (the user token lives inside the callback request and is never stored; tests check
that no print, error or response contains a token or the client secret). Not built: G8 (only github.com
exists). Also: with GitHub not configured every GitHub endpoint answers 409, never a 5xx.

**Status of the sign-in controls (2026-11-10).** Built and tested against the fake GitHub and a real
Postgres (not yet against github.com or Entra): U6 (see above), U8 in part (approval by default, the
pending list is capped; per-user limits for `open` mode are still open), U9 (the status is read on every
request, a pending user can reach only `/api/me`, rejecting or suspending deletes the user's sessions, a
pending or inactive user cannot be invited or assigned, a pending session never carries the admin role,
and approve, reject, suspend and restore are audited). The sign-in rate limiter now counts in the database
(shared by every server instance) and falls back to memory if the database fails. GitHub sign-in checks a
random `state` in a short HttpOnly cookie, uses the user token once to read the numeric account id, and
drops it; it has no PKCE (the code exchange also needs the client secret). Not built: auto-approval rules,
the re-check of an owner's repository access at sign-in (U2), stopping a suspended owner's project.

## Multi-user additions (Phase 8, D-050)

| # | Threat | Control (planned) |
|---|---|---|
| U1 | One user reads or changes another user's project | Project roles on every route; non-members get 404; the existing route-walk test is extended to the new roles |
| U2 | A contributor acts through the owner's connection after the owner lost access to the repository | The owner's access is re-checked whenever the owner signs in with GitHub, and on demand; a lost connection is marked "needs attention" and publishing is blocked; the invite screen says that inviting someone gives them branch access |
| U3 | A contributor approves | Only owner and co-owner can approve (server-checked); an optional second reviewer per project. The role rules exist in the store (`CanApprove`); the API still uses the global `devops` role until 8.2 |
| U4 | A co-owner removes or demotes the owner | Roles at or above one's own cannot be changed; only the owner transfers or deletes. In the store now: the owner cannot be demoted or removed, nothing can create a second owner, and the database refuses one |
| U5 | Admin reads users' repositories without a trace | Admin sees metadata; opening content needs support access that is audited each time |
| U6 | Account takeover through linking GitHub and Entra identities | No automatic linking; GitHub users are `github:<numeric id>`, Entra users keep the unprefixed object id (it predates the prefix); admins listed in `TF_ADMIN_USERS` are matched by id or email, never by login name |
| U7 | A user's schema or module source abuses the generator | Schemas are validated strictly (`catalog.Validate`); project schemas are visible only to their project; generated code goes only to the owner's own repository and is reviewed there |
| U8 | Abuse by creating many projects or requests | Sign-up approval by default (pending users can do nothing); per-user limits for the `open` mode (open item); the pending list is capped (`TF_SIGNUP_MAX_PENDING`) |
| U9 | A pending, rejected or suspended user gets in anyway (through an invitation, an old session, or a renamed account) | Status is checked on every request, not only at sign-in; invitations do not bypass approval; suspending deletes the user's sessions; people are identified by id, never by login name; the number of pending users is capped; approve, reject and suspend are audited |
