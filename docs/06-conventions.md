# 06 — Code conventions

How TerraFlow code is written, so changes stay consistent and reviewable. Short
and stable; the "what/why" lives in scope/tech-stack/decisions.

## General
- Smallest correct change. Don't refactor, rename or reformat unrelated code.
- Identifiers and code in English; comments may be Thai or English.
- No secrets, real resource names or prompts in code, tests, logs or fixtures.

## Go (backend)
- `gofmt`/`goimports` clean; pass `go vet` and `golangci-lint`.
- Package layout per `CLAUDE.md`. `server/internal/hcl` stays free of HTTP/DB
  concerns — it is pure parse/evaluate/generate so it can be golden-tested.
- Errors: wrap with `%w` and context; return typed/sentinel errors across
  package boundaries. API layer maps errors to stable response shapes + request
  IDs; never leak internal messages or secrets to clients.
- Logging: `slog`, structured JSON. Never log tokens, cookies, connection
  strings or full request bodies.
- SQL is hand-written in `server/internal/store/`, always parameterised (never
  concatenate user input into SQL), no ORM. Migrations via `goose`, append-only —
  never edit a released migration.
- Config from env/flags; secrets only from the host's environment (or `.env.local`
  in development), never from code or committed files.

## HCL generation (the core)
- Use `hclwrite` so comments, ordering and formatting survive and PR diffs stay
  minimal. Never build `.tf` by string concatenation.
- Emit **module calls** in the team's style and the config-driven file layout
  (`samples/`), plus the `data` blocks for cross-folder references. A raw
  `resource` block is written only for a node the developer added explicitly from
  an admin-prepared template (D-040); the generator never turns a module call into
  one. Existing files are merged, never replaced (`hcl.Apply`, D-036).
- Evaluate `var`/`locals`/`.tfvars` with `go-cty`; a value that can't be
  resolved is surfaced as `unknown`, never defaulted.
- Never interpolate unvalidated user input into HCL.
- Every generator change updates golden files deliberately and keeps the
  round-trip (parse → modify → write) byte-stable for untouched content.

## API contract
- `api/openapi.yaml` is the contract. Today the handlers and `web/src/types/` are
  written by hand, so change the spec, the handler and the types in the same
  change. Prefer additive changes. (Code generation and a drift check are planned.)

## React / TypeScript (frontend)
- `strict` TypeScript. Server state via TanStack Query; canvas state via
  Zustand. No business logic in components that belongs in hooks.
- Styling only through the design-system tokens/classes
  (`web/src/design-system`); no hardcoded hex or ad-hoc spacing. See
  `docs/05-design-system.md`.
- Resource forms are rendered from schema by `SchemaForm` — never hand-code a
  resource's fields.
- Escape user-controlled values before rendering. Derived/locked fields use the
  locked treatment and are not editable.

## Git providers
- Read and write a project's repository only through `gitprovider.Source` (via the `Repos`
  function). A handler never calls `os.ReadDir` on a project folder, runs `git`, or builds a
  GitHub URL.
- A new source must pass `gitprovidertest.Run`; a remote one also `RunBase`. Test a remote client
  against a fake that enforces the real protocol, not against a mock that returns what the code
  expects.
- Credentials: never log, print, return or wrap into an error a token, a key, an `Authorization`
  header or a request. Types that hold one print as hidden, and a test checks it.
- An unreadable repository is an error, never an empty one.

## Tests
- Go: table tests; `server/internal/hcl` carries the golden-file and round-trip
  suites (mandatory, never weakened to pass a diff). Integration tests use a
  real Postgres and a fake GitHub (`githubtest`).
- Web: Vitest for logic and schema→form rendering.
- Postgres tests need `TF_TEST_PG_DSN` (a server where the test can create and drop
  databases); without it they are skipped, so set it before you call a change tested.
- Use `samples/` as fixtures; keep them synthetic.

## Commits & branches
- Focused commits, imperative messages; conventional prefixes welcome
  (`feat:`, `fix:`, `docs:`, `test:`, `chore:`).
- Branch names: `feat/…`, `fix/…`, `docs/…`, `test/…`, `chore/…`.
