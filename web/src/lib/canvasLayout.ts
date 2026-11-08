import type { CanvasNode } from "../types/changeset";
import type { CanvasHints, ModuleSummary } from "../types/catalog";

export const LEAF_W = 230;
export const LEAF_H = 92;
const HEADER = 46;
const PAD = 16;
const GAP = 12;
const MIN_CONTAINER_W = 300;
const EMPTY_BODY_H = 56;

export type HintMap = Map<string, CanvasHints>;

export function hintMap(modules: ModuleSummary[]): HintMap {
  return new Map(modules.map((m) => [m.name, m.canvas ?? {}]));
}

export interface Placed {
  id: string;
  parentId?: string;
  /** Position relative to the parent (or absolute for top-level nodes). */
  x: number;
  y: number;
  w: number;
  h: number;
  container: boolean;
  /** Absolute position, for hit-testing. */
  ax: number;
  ay: number;
}

/** Hard-coded fallback so older schemas without x-tf-canvas still nest. */
const LEGACY_CONTAINERS = new Set(["resource-group", "virtual-network"]);

export function isContainer(moduleName: string, hints: HintMap): boolean {
  const h = hints.get(moduleName);
  if (h && h.container !== undefined) return !!h.container;
  return LEGACY_CONTAINERS.has(moduleName);
}

/** Can `child` be placed inside a node of module `parent`? */
export function canNest(child: string, parent: string, hints: HintMap): boolean {
  const allowed = hints.get(child)?.parents;
  if (!allowed || allowed.length === 0) return true;
  return allowed.includes(parent);
}

/**
 * Resolve each draft node's parent: the explicit parent_id, else (legacy) a
 * container whose name appears as one of the node's string values.
 */
export function resolveParents(nodes: CanvasNode[], hints: HintMap): Map<string, string | undefined> {
  const byId = new Map(nodes.map((n) => [n.id, n]));
  const containerByName = new Map<string, string>();
  for (const n of nodes) {
    const nm = n.values?.name;
    if (isContainer(n.module_name, hints) && typeof nm === "string" && nm) containerByName.set(nm, n.id);
  }
  const out = new Map<string, string | undefined>();
  for (const n of nodes) {
    let parent: string | undefined;
    if (n.parent_id && byId.has(n.parent_id) && n.parent_id !== n.id) {
      parent = n.parent_id;
    } else if (!n.nest_field && !n.parent_id) {
      for (const v of Object.values(n.values ?? {})) {
        const pid = typeof v === "string" ? containerByName.get(v) : undefined;
        if (pid && pid !== n.id && canNest(n.module_name, byId.get(pid)!.module_name, hints)) {
          parent = pid;
          break;
        }
      }
    }
    out.set(n.id, parent);
  }
  // Break accidental cycles (A in B in A): drop the parent of the later node.
  for (const n of nodes) {
    const seen = new Set<string>([n.id]);
    let cur = out.get(n.id);
    while (cur) {
      if (seen.has(cur)) {
        out.set(n.id, undefined);
        break;
      }
      seen.add(cur);
      cur = out.get(cur);
    }
  }
  return out;
}

/** Lay out draft nodes: containers wrap their children, children stack. */
export function layoutDrafts(nodes: CanvasNode[], hints: HintMap): Placed[] {
  const parents = resolveParents(nodes, hints);
  const children = new Map<string, CanvasNode[]>();
  const roots: CanvasNode[] = [];
  for (const n of nodes) {
    const p = parents.get(n.id);
    if (p) children.set(p, [...(children.get(p) ?? []), n]);
    else roots.push(n);
  }

  const size = new Map<string, { w: number; h: number }>();
  const measure = (n: CanvasNode): { w: number; h: number } => {
    const kids = children.get(n.id) ?? [];
    if (!isContainer(n.module_name, hints) && kids.length === 0) {
      const s = { w: LEAF_W, h: LEAF_H };
      size.set(n.id, s);
      return s;
    }
    const ks = kids.map(measure);
    const w = Math.max(MIN_CONTAINER_W, ...ks.map((k) => k.w + PAD * 2));
    const body = ks.length === 0 ? EMPTY_BODY_H : ks.reduce((a, k) => a + k.h, 0) + GAP * (ks.length - 1);
    const s = { w, h: HEADER + PAD + body + PAD };
    size.set(n.id, s);
    return s;
  };

  const placed: Placed[] = [];
  const place = (n: CanvasNode, parentId: string | undefined, x: number, y: number, ax: number, ay: number) => {
    const s = size.get(n.id)!;
    placed.push({
      id: n.id, parentId, x, y, w: s.w, h: s.h, ax, ay,
      container: isContainer(n.module_name, hints) || (children.get(n.id)?.length ?? 0) > 0,
    });
    let cy = HEADER + PAD;
    for (const k of children.get(n.id) ?? []) {
      place(k, n.id, PAD, cy, ax + PAD, ay + cy);
      cy += size.get(k.id)!.h + GAP;
    }
  };

  for (const r of roots) measure(r);
  for (const r of roots) place(r, undefined, r.position_x, r.position_y, r.position_x, r.position_y);
  return placed;
}

/** A node that was added without a spot of its own (position 0,0) and edits nothing in the repository. */
export function needsAutoPlace(n: CanvasNode): boolean {
  return !n.existing && !n.parent_id && !n.nest_field && n.position_x === 0 && n.position_y === 0;
}

/**
 * Give new nodes that have no spot a free one: in a column to the right of
 * everything already drawn (origin.x), stacked with their real heights, so they
 * never sit on top of the repository's resources or each other. Nodes the user
 * dragged, dropped or that edit a repository block keep their position.
 */
export function placeAutoRoots(nodes: CanvasNode[], placed: Placed[], origin: { x: number; y: number }): CanvasNode[] {
  const heights = new Map(placed.map((p) => [p.id, p.h]));
  let y = origin.y;
  const moved = new Map<string, { x: number; y: number }>();
  for (const n of nodes) {
    if (!needsAutoPlace(n)) continue;
    moved.set(n.id, { x: origin.x, y });
    y += (heights.get(n.id) ?? LEAF_H) + GAP * 2;
  }
  if (moved.size === 0) return nodes;
  return nodes.map((n) => (moved.has(n.id) ? { ...n, position_x: moved.get(n.id)!.x, position_y: moved.get(n.id)!.y } : n));
}

/** Deepest container under (x, y) that accepts `childModule`; undefined = none. */
export function findDropTarget(
  placed: Placed[],
  nodes: CanvasNode[],
  x: number,
  y: number,
  childModule: string,
  hints: HintMap,
): CanvasNode | undefined {
  const byId = new Map(nodes.map((n) => [n.id, n]));
  let best: { node: CanvasNode; depth: number } | undefined;
  const depthOf = (p: Placed): number => {
    let d = 0;
    for (let cur = p; cur.parentId; d++) cur = placed.find((q) => q.id === cur.parentId)!;
    return d;
  };
  for (const p of placed) {
    if (!p.container) continue;
    if (x < p.ax || x > p.ax + p.w || y < p.ay || y > p.ay + p.h) continue;
    const n = byId.get(p.id)!;
    if (!isContainer(n.module_name, hints) || !canNest(childModule, n.module_name, hints)) continue;
    const depth = depthOf(p);
    if (!best || depth > best.depth) best = { node: n, depth };
  }
  return best?.node;
}

/** Ids of a node and all its descendants (for cascading deletes). */
export function descendantIds(nodes: CanvasNode[], rootId: string): Set<string> {
  const out = new Set<string>([rootId]);
  let grew = true;
  while (grew) {
    grew = false;
    for (const n of nodes) {
      if (n.parent_id && out.has(n.parent_id) && !out.has(n.id)) {
        out.add(n.id);
        grew = true;
      }
    }
  }
  return out;
}

export const BLOCK_COLORS = [
  { key: "cyan", label: "Cyan", value: "#06b6d4" },
  { key: "blue", label: "Blue", value: "#3b82f6" },
  { key: "violet", label: "Violet", value: "#8b5cf6" },
  { key: "rose", label: "Rose", value: "#f43f5e" },
  { key: "amber", label: "Amber", value: "#f59e0b" },
  { key: "green", label: "Green", value: "#22c55e" },
  { key: "slate", label: "Slate", value: "#64748b" },
] as const;

export function colorValue(key: string | undefined): string {
  return BLOCK_COLORS.find((c) => c.key === key)?.value ?? BLOCK_COLORS[0].value;
}
