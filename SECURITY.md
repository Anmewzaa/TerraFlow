# Security Policy

TerraFlow is an internal tool handling access to infrastructure change. Report
suspected security issues **privately** to the team's security contact
(`<fill in: internal security channel / email>`), not in a public issue or chat.

Include enough detail to reproduce, the affected version/commit, and any
mitigation you have identified.

When reporting, **redact** everything sensitive: Entra tokens, session cookies,
ADO tokens, connection strings, real resource names, and any Azure identifiers.

## Handling rules (also enforced in code review)

- TerraFlow holds no Azure write credentials and makes no Azure/ARM calls.
- Service access (Azure DB, Key Vault, ADO) is via one Workload Identity with
  least privilege; ADO is scoped to Contribute + Create branch on the target
  repo only.
- No secrets in code, logs, fixtures, screenshots or the UI. The only Key Vault
  secret is the session-cookie encryption key.
- User-controlled values are escaped before rendering, and are never
  interpolated into generated HCL without validation.
- Branch policy is never bypassed.
