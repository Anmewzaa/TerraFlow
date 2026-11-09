## Summary

<!-- What changed and why. -->

## Type of change

- [ ] Feature
- [ ] Bug fix
- [ ] Refactor
- [ ] Tests
- [ ] Documentation
- [ ] Tooling / CI

## Affected areas

- [ ] HCL parse / inventory (`server/internal/hcl`)
- [ ] HCL generator (module calls, layout)
- [ ] Module catalog / schema → form (`server/internal/catalog`, `web`)
- [ ] Canvas / editing (`web`)
- [ ] Review / approve / branch (`server/internal/localrepo`, `server/internal/ado`)
- [ ] Auth / Entra BFF / roles (`server/internal/auth`)
- [ ] Store / Postgres / migrations (`server/internal/store`)
- [ ] API contract (`api/openapi.yaml`)
- [ ] Design system / UI (`web/src/design-system`)
- [ ] Docs / samples

## Hard-rule check

- [ ] Change does nothing past branch creation (no PR/merge/pipeline/plan/apply).
- [ ] No Azure/ARM calls; inventory still comes from the repo, not Azure.
- [ ] No credentials stored in the portal; Workload Identity scope unchanged or
      justified.
- [ ] Resource forms stay schema-driven (no hardcoded fields).
- [ ] Output is still module calls in the team's layout (raw resources only when a developer adds one explicitly, D-040).
- [ ] Unresolved values render as `unknown`, not `0`/blank.

## Compatibility & migration

- Generated HCL impact: <!-- none / golden files updated (why) -->
- OpenAPI impact: <!-- none / additive / breaking -->
- DB migration: <!-- no / yes, describe -->

## Test plan

- [ ] `go vet ./... && go test ./...` (incl. golden + round-trip)
- [ ] `npm --prefix web run build && npm --prefix web test`
- [ ] Migration up/down on a throwaway DB (if schema changed)
- [ ] Manual verification:

## Reviewer notes / residual risk

<!-- Focused review areas, known limits, follow-ups. -->

## Linked issues

<!-- e.g. Fixes #123 -->
