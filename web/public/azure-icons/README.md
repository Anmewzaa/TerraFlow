# Azure resource icons

Official **Azure architecture icons** from Microsoft, self-hosted here so the
portal works air-gapped. Used to mark each resource type on the canvas, in the
module catalog, and in the inventory.

## Terms (important)
- These are Microsoft's icons. Use them **unmodified** — do not recolor, rotate,
  redraw, or add effects. TerraFlow encodes the resource *domain* with a separate
  coloured accent bar, never by altering the icon.
- Do not use them to imply Microsoft endorsement.
- Source: Microsoft "Azure architecture icons" set. Refresh from the official
  download; keep the mapping below in sync.

## Mapping (file ← official icon)
| File | Official icon |
|---|---|
| resource-group.svg | general / Resource-Groups |
| virtual-network.svg | networking / Virtual-Networks |
| subnet.svg | networking / Subnet |
| network-security-group.svg | networking / Network-Security-Groups |
| key-vault.svg | security / Key-Vaults |
| storage-account.svg | storage / Storage-Accounts |
| private-dns-zone.svg | networking / DNS-Zones |
| private-endpoint.svg | other / Private-Endpoints |
| managed-identity.svg | identity / Managed-Identities |

Each new module the DevOps team adds should map to one icon here; add the file
and a row above rather than reusing an unrelated icon.
