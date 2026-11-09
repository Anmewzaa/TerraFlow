import { describe, expect, it } from "vitest";
import { computeLayout } from "./elk";
import { layoutDrafts, placeAutoRoots, hintMap } from "../lib/canvasLayout";
import type { ProjectGraph, GraphNode } from "../types/graph";
import type { CanvasNode } from "../types/changeset";

// Phase 7: the canvas must stay usable with 80+ nodes (docs/05-design-system.md
// designs for ~80 nodes). The limits are generous so a slow CI machine does not
// flake; a real regression (a quadratic loop) misses them by a wide margin.

function graphOf(folders: number, perFolder: number): ProjectGraph {
  const gs: ProjectGraph = { project: "P", env: "dev", folders: [], externals: [] };
  for (let f = 0; f < folders; f++) {
    const nodes: GraphNode[] = [];
    const rg: GraphNode = { id: `f${f}.module.rg`, folder: `f${f}`, kind: "module", type: "resource-group", domain: "neutral", name: `rg-${f}`, nameState: "resolved", container: null, inputs: {} };
    nodes.push(rg);
    for (let i = 0; i < perFolder - 1; i++) {
      nodes.push({ id: `f${f}.module.m${i}`, folder: `f${f}`, kind: "module", type: i % 3 === 0 ? "storage-account" : "key-vault", domain: "data", name: `res-${f}-${i}`, nameState: "resolved", container: rg.id, inputs: {} });
    }
    // each resource refers to the one before it (the first has nothing to refer to)
    const edges = nodes.slice(2).map((n, i) => ({ from: n.id, to: nodes[i + 1].id, kind: "module-output" as const }));
    gs.folders.push({ name: `f${f}`, nodes, edges });
  }
  return gs;
}

describe("canvas layout with references across containers", () => {
  it("lays out an edge between resources of two different resource groups", async () => {
    const g = graphOf(1, 4);
    const other: GraphNode = { id: "f0.module.rg2", folder: "f0", kind: "module", type: "resource-group", domain: "neutral", name: "rg-2", nameState: "resolved", container: null, inputs: {} };
    const child: GraphNode = { id: "f0.module.c2", folder: "f0", kind: "module", type: "key-vault", domain: "data", name: "kv", nameState: "resolved", container: other.id, inputs: {} };
    g.folders[0].nodes.push(other, child);
    g.folders[0].edges!.push({ from: child.id, to: "f0.module.m1", kind: "module-output" }); // across the two groups
    const out = await computeLayout(g);
    expect(out.nodes.find((n) => n.id === child.id)).toBeTruthy();
    expect(out.edges.some((e) => e.from === child.id)).toBe(true);
  });
});

describe("canvas performance", () => {
  for (const [folders, per] of [[5, 20], [10, 20]] as const) {
    it(`lays out ${folders * per} nodes in ${folders} folders`, async () => {
      const g = graphOf(folders, per);
      const t0 = performance.now();
      const out = await computeLayout(g);
      const ms = performance.now() - t0;
      expect(out.nodes.length).toBeGreaterThanOrEqual(folders * per);
      expect(out.nodes.every((n) => Number.isFinite(n.x) && Number.isFinite(n.y))).toBe(true);
      console.log(`layout of ${folders * per} nodes: ${Math.round(ms)} ms`);
      expect(ms).toBeLessThan(10_000);
    }, 30_000);
  }

  it("places 200 draft nodes quickly and without overlap of siblings", () => {
    const nodes: CanvasNode[] = [];
    for (let i = 0; i < 200; i++) {
      nodes.push({ id: `n${i}`, module_name: "storage-account", module_version: "v1", instance_name: `sa_${i}`, target_folder: "storage", values: {}, position_x: 0, position_y: 0 } as CanvasNode);
    }
    const t0 = performance.now();
    const placed = layoutDrafts(nodes, hintMap([]));
    const spread = placeAutoRoots(nodes, placed, { x: 1000, y: 0 });
    const ms = performance.now() - t0;
    expect(placed).toHaveLength(200);
    expect(ms).toBeLessThan(500);
    const keys = new Set(spread.map((n) => `${n.position_x}:${n.position_y}`));
    expect(keys.size).toBe(200); // nothing is stacked on the same spot
  });
});
