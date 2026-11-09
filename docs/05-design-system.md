# 05 — Design system ("Harbor")

Status: in use (updated 2026-11-09). The colours are the **Aurora** palette
(cyan on black, D-028), which supersedes the Sapphire & Gold proposal; the Harbor
structure, tokens and interaction contract below are unchanged.

TerraFlow's UI follows one design system so that 80-node canvases, dense tables
and review screens stay readable and consistent. It has its own identity rather
than a generic default, and it is driven entirely by tokens.

The implementation lives in [`web/src/design-system/`](../web/src/design-system/README.md).
This document is the rationale and the contract.

## Identity

| | |
|---|---|
| System | **Harbor** |
| Palette | **Aurora** (D-028, `web/src/design-system/aurora.css`, loaded after the Harbor tokens): dark = black with cyan; light = pale cyan-white with cyan-700 actions. Gold remains only as the "draft" accent. *Originally proposed: Sapphire & Gold (deep bank blue); superseded.* |
| Tagline | Compose infrastructure. Keep the repo in control. |
| UI type | Inter (SIL OFL) |
| Code type | JetBrains Mono (SIL OFL) — every identifier, module source and HCL fragment |

**Historical rationale for the original Sapphire & Gold proposal (superseded by D-028):** why a deep bank blue, not Azure blue. The bank's own identity is blue, so a
deep, saturated sapphire fits the house. It is deliberately deeper and warmer
than Azure's stock `#0078D4`, so the console does not read as a default Microsoft
template. A single gold accent (selection, brand mark) gives it a bank feel
without a second loud color. The brand is **not** the bank's real logo or
wordmark — only the colour family — so nothing here impersonates the bank.
Status colors are teal/green/amber/red and the informational tone is **teal**,
not blue, so a status color never collides with the brand blue.

## Token architecture

Three layers, one direction. Feature views only ever touch the semantic layer.

```
primitive (raw)        semantic (meaning)          component
--tf-copper        ->  --tf-color-accent       ->  .tf-badge[data-tone=info]
--tf-stone-50      ->  --tf-color-canvas       ->  body background
--tf-graphite-800  ->  --tf-color-action       ->  .tf-btn
```

- Rebranding, or dropping in a bank-mandated palette, is a single-file edit to
  `brand-tokens.css`.
- No hex values in feature code. A review that finds one rejects it.

## Light and dark

Light and dark share identical geometry; only semantic colors change.
`data-theme` on `:root` selects the mode; its absence follows the OS
(`prefers-color-scheme`). Only the chosen mode is persisted, under
`localStorage` key `tf-theme`. Engineers work in dark most of the day, so
dark is a first-class target, not an afterthought — the resource-domain colors
have tuned dark variants.

## The interaction contract

These are the rules that keep the UI from drifting into generic territory.

1. **Dense and scannable before pretty.** 14px base, 13px tables, 34px controls,
   6–12px radii. Design for ~80 nodes and 40-character names, not a demo of 5.
2. **Honest unknown states.** The inventory evaluates `var`/`locals`/`.tfvars`
   and resolution can fail. An unresolved value renders as a muted `unknown`
   (`value()` helper), never as `0` or blank.
3. **Real Azure icons, unmodified.** Each resource type is marked with the
   official Azure architecture icon (self-hosted under
   `web/public/azure-icons/`, so it works air-gapped). The icons are used
   unmodified — the resource *domain* is encoded by a separate coloured accent
   bar, never by recolouring the icon. See that folder's README for terms.
4. **The identifier is sacred.** Resource names, module sources and HCL are
   monospace (`ident()` / `.tf-ident`), selectable, and wrap or scroll — never
   silently truncated.
5. **Read-only looks read-only.** Derived/locked fields (for example the RG of a
   chosen VNet) use the hatched `.tf-locked` treatment, so they read as inert,
   not merely greyed out.
6. **Status tone is centralized.** `toneForStatus()` maps lifecycle words
   (draft, pending review, approved, rejected, locked, external) to five tones,
   so the same state looks the same on the canvas, in lists and in review.
7. **Resource-domain color is semantic, and validated.** Four domains carry a
   hue (`network`, `compute`, `data`, `security`), chosen from an Okabe-Ito base
   and checked for colorblind separation and surface contrast in both themes with
   the `dataviz` skill — not picked by eye. Resource groups, `integration` and
   external `data`/hub references use the neutral and are told apart by icon +
   label. Color is always secondary: every node has an icon and a monospace label,
   so identity never depends on color alone.
8. **Restraint and accessibility.** One accent. Motion 110–160ms and removed
   under `prefers-reduced-motion`. A visible focus ring on every control.

## Component vocabulary

A structure used by two or more views becomes a helper in `components.ts`; a
one-off stays in its view.

| Helper / class | Use |
|---|---|
| `badge(label, tone?)` | Any lifecycle/status signal |
| `ident(name)` | A resource name, module source, HCL token |
| `value(v)` | A value that may be unknown (honest-unknown) |
| `panel(title, body, meta?)` | Standard section container |
| `metricTile(label, value, tone?)` | A single number with a status accent |
| `tableWrap(markup)` | Scrollable, focusable table region |
| `emptyState(message, hint?)` | Empty list / no-results |
| `.tf-locked` | A derived, non-editable field |
| `.tf-dot[data-domain]` | Resource-domain color dot |

## Where it plugs in

- Every view uses this system. Tailwind CSS v4 is loaded as utilities only
  (no preflight) so it does not restyle Harbor components (D-026). No third-party
  component library is used.

## Preview

`web/design-preview.html` is a standalone page from the original Sapphire & Gold
proposal. It does **not** show the Aurora palette and is kept only as history;
judge the current look in the running app (see the screenshots in `docs/screenshots/`).
