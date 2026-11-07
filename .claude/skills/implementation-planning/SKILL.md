---
name: implementation-planning
description: Plan a TerraFlow change before editing — bound the scope, check it against the hard rules and compatibility surfaces, and pick the simplest implementation. Use before changing the HCL parser/generator, API contract, DB schema, auth, or any behavior.
---

# Implementation planning

Use before editing when a change affects behavior, so the implementation stays
simple and never quietly breaks a contract. Counters the usual failure modes:
premature coding, speculative abstraction, broad rewrites, vague "done".

## 1. Restate the change
- Say the concrete behavior/artifact being changed in one sentence.
- List assumptions. If two readings differ materially, **stop and ask**.

## 2. Check it against the hard rules (`CLAUDE.md`)
Confirm the change does not: go past branch creation; call Azure; store
credentials; hardcode form fields; emit raw resource blocks; or render
unresolved values as `0`/blank. If it seems to need one of those, the design is
wrong — reconsider or ask.

## 3. Identify the touched compatibility surface
Generated HCL (style/layout/golden files) · `api/openapi.yaml` · Postgres
schema/migrations + canvas JSONB · ADO interaction + Workload Identity scope ·
Entra group→role mapping.

## 4. Decide the shape
- Smallest change within the owning package/dir; no new abstraction for one
  caller.
- Generated HCL: keep diffs minimal; update golden files deliberately and say
  why.
- OpenAPI: prefer additive; regenerate server + TS client together.
- DB: add a `goose` migration; never edit a released one; add backward-read if
  the JSONB shape changes.
- User input that reaches HCL or the DOM must be validated/escaped.

## 5. Plan verification
Name the checks you'll run (see the `change-verification` skill) and the test
that proves the requested behavior — for generator work, which golden/round-trip
cases.

## Stop and confirm when
The change alters released generated-HCL style broadly, the OpenAPI contract in
a breaking way, a released migration, the identity's scope, or the role mapping;
or there are two paths with materially different cost.

## Handoff
State what changed, which compatibility surface it touched and the decision, and
what verification ran.
