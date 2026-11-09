# TerraFlow design system — "Harbor"

The reusable visual foundation for the TerraFlow portal. It keeps the UI dense,
scannable and governance-oriented, with its own identity. The palette is **Aurora** (cyan on black, `aurora.css`, D-028), loaded after `brand-tokens.css`; the original Sapphire & Gold values in `brand-tokens.css` are the base it overrides.

> Why this exists: an internal infra console lives or dies on information
> density and honest state, not on decoration. These files make "dense, honest,
> restrained" the default so no feature view has to re-decide it.

## Files

- `brand-tokens.css` — the single source of truth: primitive values, then
  semantic tokens (`--tf-color-*`, spacing, radius, type) with light/dark.
  Import once through the app stylesheet. **Never** duplicate a hex value in a
  feature view; use a semantic token.
- `aurora.css` — re-defines the semantic colour tokens (dark and light) for the
  current palette; loaded after `brand-tokens.css`. Change colours here (D-028).
- `shell.css` — layout of the app shell (nav rail, top bar, Home).
- `tokens.css` — base element styles and component classes
  (`.tf-panel`, `.tf-badge`, `.tf-stat`, `.tf-ident`, …) built only from tokens.
- `brand-identity.ts` — names, palette anchors, resource-domain color meanings
  and the design principles. Import copy/meaning from here, not from literals.
- `components.ts` — the component vocabulary (`badge`, `panel`, `metricTile`,
  `ident`, `value`, `emptyState`, `tableWrap`) and `toneForStatus`.
- `index.ts` — the public import surface. Import from `./design-system`, not
  from individual files.

## Token layers

```
primitive        semantic                     usage
--tf-blue-700 -> --tf-color-action        ->  class .tf-btn
--tf-gold    ->  --tf-color-accent        ->  selection / brand mark
--tf-gray-50  -> --tf-color-canvas        ->  body background
```

Feature code touches only the right-hand side. Rebranding = editing
`brand-tokens.css` once.

## Visual & interaction contract

- **Theme:** light and dark share identical geometry; only semantic colors
  change. `data-theme` on `:root` selects the mode (`light` / `dark`); absence
  follows the OS. The chosen mode is the only thing persisted
  (`localStorage` key `tf-theme`).
- **Density:** base font 14px, table rows 13px, control height 34px, tight radii
  (6–12px). The canvas is expected to hold ~80 nodes with 40-character names.
- **Identifiers:** every resource name, module source and HCL fragment is
  monospace (`ident()` / `.tf-ident`), selectable, and wraps or scrolls — never
  silently truncated.
- **Honest unknown:** use `value()` — an unresolved value renders as a muted
  `unknown`, never as `0` or blank. This matters because the inventory evaluates
  `var`/`locals`/`.tfvars` and resolution can genuinely fail.
- **Status tone is centralized:** `toneForStatus()` maps lifecycle words
  (draft, pending review, approved, rejected, locked, external) to one of five
  tones so the same state looks the same on the canvas, in lists and in review.
- **Read-only looks read-only:** derived/locked fields (e.g. the RG of a chosen
  VNet) are visibly inert, not just disabled text.
- **Motion:** 110–160ms, `prefers-reduced-motion` removes it. Keyboard focus is
  always visible (blue focus ring).

## Resource-domain colors

Node color is **semantic**, not decorative. Four domains carry a hue —
`network`, `compute`, `data`, `security` — chosen from an Okabe-Ito base and
**validated colorblind-safe and contrast-checked in both themes** with the
`dataviz` skill. Resource groups (the container frame), `integration`, and
external `data`/hub references use the **neutral** and are identified by their
icon + label. Color is always a secondary cue: every node also has an icon and a
monospace label, so identity never depends on color alone. Legend is always
present.

## Fonts

Inter (UI) and JetBrains Mono (identifiers/HCL), both SIL OFL, self-hosted and
pinned. No webfont CDN at runtime, because production is air-gapped.

## Resource icons

Each resource type is marked with the official Azure architecture icon, kept in
`web/public/azure-icons/` and used **unmodified** (domain is shown by a coloured
accent bar, not by recolouring the icon). See that folder's README for the
mapping and Microsoft's usage terms.
