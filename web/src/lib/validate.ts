import type { CanvasNode } from "../types/changeset";
import type { ModuleSummary } from "../types/catalog";

export interface Issue {
  /** Short, user-facing text. */
  text: string;
  /** Input name the issue is about, if any. */
  field?: string;
}

function isEmpty(v: unknown): boolean {
  if (v === undefined || v === null) return true;
  if (typeof v === "string") return v.trim() === "";
  if (Array.isArray(v)) return v.length === 0;
  return false;
}

/**
 * What is still missing on a draft node. A dropped resource is always placed;
 * the canvas marks it with a red border until these are resolved in the panel.
 */
export function nodeIssues(
  node: CanvasNode,
  parentId: string | undefined,
  module: ModuleSummary | undefined,
): Issue[] {
  const issues: Issue[] = [];
  const canvas = module?.canvas;
  if (canvas?.parent_required && !parentId) {
    const where = (canvas.parents ?? []).join(" or ") || "a parent resource";
    issues.push({ text: `Drag it into ${where}` });
  }
  for (const field of module?.required ?? []) {
    // The parent fills this field in when HCL is generated.
    if (canvas?.parent_ref?.field === field && parentId) continue;
    if (isEmpty(node.values?.[field])) issues.push({ text: `${field.replace(/_/g, " ")} is required`, field });
  }
  return issues;
}

/** Defaults worth writing into a new node so the generated HCL is complete. */
export function defaultValues(properties: Record<string, { default?: unknown }> | undefined): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [k, p] of Object.entries(properties ?? {})) {
    const d = p?.default;
    if (d === undefined || d === null || d === "") continue;
    if (Array.isArray(d) && d.length === 0) continue;
    if (typeof d === "object" && !Array.isArray(d) && Object.keys(d as object).length === 0) continue;
    out[k] = d;
  }
  return out;
}
