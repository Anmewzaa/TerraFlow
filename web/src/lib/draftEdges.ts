import type { CanvasNode } from "../types/changeset";

export interface RefEdge {
  id: string;
  from: string; // the resource whose output is used
  to: string; // the draft node that uses it
  /** The kind of link, when the reference sits in an input that asks for one (private_link...): drawn dashed. */
  link?: string;
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
export function referenceEdges(drafts: CanvasNode[], graphIds: Set<string>, linkFields?: Map<string, Record<string, string>>): RefEdge[] {
  const draftIds = new Set(drafts.map((d) => d.id));
  const byInstance = new Map(drafts.map((d) => [`${d.target_folder}/${d.instance_name}`, d.id]));
  const seen = new Set<string>();
  const out: RefEdge[] = [];
  for (const d of drafts) {
    const kinds = linkFields?.get(d.module_name) ?? {};
    const entries = Object.entries(d.values ?? {}).flatMap(([field, v]) => strings(v).map((s) => [s, kinds[field]] as const));
    for (const [s, link] of entries) {
      for (const m of s.matchAll(MODULE_REF)) {
        const name = m[1];
        const repoId = `${d.target_folder}.module.${name}`;
        const from =
          byInstance.get(`${d.target_folder}/${name}`) ??
          (graphIds.has(repoId) || draftIds.has(repoId) ? repoId : null);
        if (!from || from === d.id) continue;
        const id = `ref:${from}->${d.id}`;
        if (seen.has(id)) {
          // The same pair may be referenced by a plain field and a link field: the link wins.
          const prior = out.find((e) => e.id === id);
          if (prior && link && !prior.link) prior.link = link;
          continue;
        }
        seen.add(id);
        out.push({ id, from, to: d.id, ...(link ? { link } : {}) });
      }
    }
  }
  return out.sort((a, b) => a.id.localeCompare(b.id));
}
