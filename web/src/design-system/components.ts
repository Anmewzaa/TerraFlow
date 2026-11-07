// Component vocabulary. Two or more views sharing a structure earns a helper
// here; a one-off stays in its view. Status tone is centralized so an operator
// risk signal looks identical everywhere it appears.
export type Tone = "good" | "bad" | "warn" | "info" | "neutral";

const GOOD = ["approved", "ready", "healthy", "ok", "enabled", "resolved", "merged", "active"];
const BAD = ["rejected", "failed", "error", "disabled", "unresolved", "conflict", "missing"];
const WARN = ["draft", "pending", "pending review", "changed", "locked", "unknown", "stale"];
const INFO = ["external", "data-source", "read-only", "reference"];

export function toneForStatus(value: unknown): Tone {
  const s = String(value ?? "").toLowerCase();
  if (GOOD.includes(s)) return "good";
  if (BAD.includes(s)) return "bad";
  if (WARN.includes(s)) return "warn";
  if (INFO.includes(s)) return "info";
  return "neutral";
}

export function esc(value: unknown): string {
  return String(value ?? "")
    .replaceAll("&", "&amp;").replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;").replaceAll('"', "&quot;");
}

export function badge(label: unknown, tone: Tone = toneForStatus(label)): string {
  return `<span class="tf-badge" data-tone="${tone}">${esc(label ?? "unknown")}</span>`;
}

// A resource/module identifier. Always monospace, never silently truncated.
export function ident(name: unknown): string {
  return `<code class="tf-ident">${esc(name)}</code>`;
}

export function panel(title: string, body: string, meta = ""): string {
  const head = title
    ? `<div class="tf-panel__head"><h3>${esc(title)}</h3>${meta ? `<span class="tf-subtle">${esc(meta)}</span>` : ""}</div>`
    : "";
  return `<section class="tf-panel">${head}${body}</section>`;
}

export function metricTile(label: string, value: unknown, tone: Tone = "neutral"): string {
  return `<section class="tf-stat" data-tone="${tone}"><span>${esc(label)}</span><strong>${esc(value)}</strong></section>`;
}

// Honest-unknown contract: a value that could not be resolved is stated as
// unknown, not defaulted to zero or blank.
export function value(v: unknown): string {
  return v === null || v === undefined || v === ""
    ? `<span class="tf-unknown">unknown</span>`
    : esc(v);
}

export function emptyState(message: string, hint = ""): string {
  return `<div class="tf-empty"><p>${esc(message)}</p>${hint ? `<p class="tf-subtle">${esc(hint)}</p>` : ""}</div>`;
}

export function tableWrap(markup: string): string {
  return `<div class="tf-table-wrap" tabindex="0">${markup}</div>`;
}
