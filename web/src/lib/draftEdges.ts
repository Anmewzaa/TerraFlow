import type { CanvasNode } from "../types/changeset";

export interface RefEdge {
  id: string;
  from: string; // the resource whose output is used
  to: string; // the draft node that uses it
}

const MODULE_REF = /\bmodule\.([A-Za-z_][A-Za-z0-9_-]*)/g;

/** Every string inside a value, including the text of {"$expr": "..."} markers. */
function strings(v: unknown, out: string[] = []): string[] {
  if (typeof v === "string") out.push(v);
  else if (Array.isArray(v)) v.forEach((x) => strings(x, out));
  else if (v && typeof v === "object") Object.values(v).forEach((x) => strings(x, out));
  return out;
}

/**
 * Edges for `module.<name>...` references found in draft nodes' values.
 * <name> is looked up in the node's own folder, among the repository's
 * resources (graphIds) and the other draft nodes. A reference that matches
 * nothing makes no edge. Output is sorted so it is stable between renders.
 */
export function referenceEdges(drafts: CanvasNode[], graphIds: Set<string>): RefEdge[] {
  const draftIds = new Set(drafts.map((d) => d.id));
  const byInstance = new Map(drafts.map((d) => [`${d.target_folder}/${d.instance_name}`, d.id]));
  const seen = new Set<string>();
  const out: RefEdge[] = [];
  for (const d of drafts) {
    for (const s of strings(d.values)) {
      for (const m of s.matchAll(MODULE_REF)) {
        const name = m[1];
        const repoId = `${d.target_folder}.module.${name}`;
        const from =
          byInstance.get(`${d.target_folder}/${name}`) ??
          (graphIds.has(repoId) || draftIds.has(repoId) ? repoId : null);
        if (!from || from === d.id) continue;
        const id = `ref:${from}->${d.id}`;
        if (seen.has(id)) continue;
        seen.add(id);
        out.push({ id, from, to: d.id });
      }
    }
  }
  return out.sort((a, b) => a.id.localeCompare(b.id));
}
