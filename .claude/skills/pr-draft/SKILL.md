---
name: pr-draft
description: Produce a ready-to-paste TerraFlow PR title and description after a substantive change, from local git state. Use when wrapping up a code/test/behavior change and you need the PR block.
---

# PR draft

Produce a PR block after substantive work (code, tests, generator, schema, API,
behavior docs). Skip for trivial or docs-only/metadata-only changes, or if asked
not to.

## Collect from git (don't ask the user)
```bash
git rev-parse --abbrev-ref HEAD
git status -sb
git ls-files --others --exclude-standard     # untracked — call these out
git diff --stat; git diff --stat --cached
BASE=$(git rev-parse --abbrev-ref --symbolic-full-name @{upstream} 2>/dev/null || echo origin/main)
git log --oneline --no-merges $(git merge-base "$BASE" HEAD)..HEAD
```

## Build the block
- Infer type from touched paths: feature / fix / refactor / test / docs / tooling.
- Lead verb: feature→adds, fix→fixes, refactor/perf→improves, docs/tooling→updates.
- Flag compatibility risk only if the diff touches generated-HCL style/layout,
  `api/openapi.yaml`, a released migration or the JSONB shape, the ADO/identity
  scope, or the Entra role mapping.
- Branch: keep the current feature branch, else suggest `feat|fix|docs|test|chore/<slug>`.

## Output
```markdown
# Pull Request Draft

## Branch
git checkout -b <type/slug>

## Title
<imperative one-liner, optional prefix feat:/fix:/docs:/test:/chore:>

## Description
This pull request <adds/fixes/improves/updates> ...
<what changed, why, hard-rule/compatibility notes, residual risk>
```
Also fill `.github/PULL_REQUEST_TEMPLATE.md`'s hard-rule and compatibility
sections honestly.
