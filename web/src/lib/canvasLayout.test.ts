import { describe, expect, it } from "vitest";
import { canNest, descendantIds, findDropTarget, hintMap, layoutDrafts, needsAutoPlace, placeAutoRoots, resolveParents } from "./canvasLayout";
import type { CanvasNode } from "../types/changeset";
import type { ModuleSummary } from "../types/catalog";

const mod = (name: string, canvas: ModuleSummary["canvas"]): ModuleSummary => ({
  name, title: name, description: "", icon: "", versions: ["v1"], latest_version: "v1", canvas,
});
const hints = hintMap([
  mod("resource-group", { container: true }),
  mod("virtual-network", { container: true, parents: ["resource-group"] }),
  mod("subnet", { container: true, parents: ["virtual-network"], parent_required: true, nest: { field: "subnets" } }),
  mod("private-endpoint", { parents: ["subnet"], parent_required: true }),
  mod("key-vault", { parents: ["resource-group"] }),
]);

const n = (id: string, module_name: string, extra: Partial<CanvasNode> = {}): CanvasNode => ({
  id, module_name, module_version: "v1", instance_name: id, target_folder: "x",
  values: {}, position_x: 100, position_y: 50, ...extra,
});

describe("canvas layout", () => {
  const nodes = [
    n("rg", "resource-group"),
    n("vnet", "virtual-network", { parent_id: "rg" }),
    n("snet", "subnet", { parent_id: "vnet", nest_field: "subnets" }),
    n("pe", "private-endpoint", { parent_id: "snet" }),
  ];

  it("nests rg > vnet > subnet > private endpoint and wraps children", () => {
    const placed = layoutDrafts(nodes, hints);
    const by = Object.fromEntries(placed.map((p) => [p.id, p]));
    expect(by.pe.parentId).toBe("snet");
    expect(by.snet.parentId).toBe("vnet");
    expect(by.vnet.parentId).toBe("rg");
    expect(by.rg.parentId).toBeUndefined();
    // each container is taller than the child it wraps
    expect(by.rg.h).toBeGreaterThan(by.vnet.h);
    expect(by.vnet.h).toBeGreaterThan(by.snet.h);
    expect(by.snet.h).toBeGreaterThan(by.pe.h);
    // absolute position of the deepest child lies inside the root rect
    expect(by.pe.ax).toBeGreaterThan(by.rg.ax);
    expect(by.pe.ay + by.pe.h).toBeLessThanOrEqual(by.rg.ay + by.rg.h);
  });

  it("only allows children where the schema says", () => {
    expect(canNest("subnet", "virtual-network", hints)).toBe(true);
    expect(canNest("subnet", "resource-group", hints)).toBe(false);
    expect(canNest("private-endpoint", "subnet", hints)).toBe(true);
    expect(canNest("private-endpoint", "virtual-network", hints)).toBe(false);
    expect(canNest("resource-group", "anything", hints)).toBe(true);
  });

  it("finds the deepest accepting container under the cursor", () => {
    const placed = layoutDrafts(nodes, hints);
    const snet = placed.find((p) => p.id === "snet")!;
    const target = findDropTarget(placed, nodes, snet.ax + 10, snet.ay + 10, "private-endpoint", hints);
    expect(target?.id).toBe("snet");
    // a private endpoint dropped over the vnet header area is not accepted there
    const vnet = placed.find((p) => p.id === "vnet")!;
    expect(findDropTarget(placed, nodes, vnet.ax + 5, vnet.ay + 5, "private-endpoint", hints)).toBeUndefined();
  });

  it("falls back to name matching for resources that reference a container", () => {
    const list = [
      n("rg", "resource-group", { values: { name: "rg-a" } }),
      n("kv", "key-vault", { values: { name: "kv1", resource_group_name: "rg-a" } }),
    ];
    expect(resolveParents(list, hints).get("kv")).toBe("rg");
  });

  it("cascades deletes to descendants and survives parent cycles", () => {
    expect([...descendantIds(nodes, "vnet")].sort()).toEqual(["pe", "snet", "vnet"]);
    const cyc = [n("a", "resource-group", { parent_id: "b" }), n("b", "resource-group", { parent_id: "a" })];
    const parents = resolveParents(cyc, hints);
    expect([parents.get("a"), parents.get("b")].filter(Boolean).length).toBeLessThan(2);
    expect(layoutDrafts(cyc, hints)).toHaveLength(2);
  });
});

describe("auto placement of new nodes", () => {
  const fresh = (id: string, extra: Partial<CanvasNode> = {}) => n(id, "key-vault", { position_x: 0, position_y: 0, ...extra });

  it("only nodes without a spot, a parent, or a repository block are auto-placed", () => {
    expect(needsAutoPlace(fresh("a"))).toBe(true);
    expect(needsAutoPlace(fresh("a", { position_x: 40 }))).toBe(false); // dragged or dropped
    expect(needsAutoPlace(fresh("a", { parent_id: "rg" }))).toBe(false);
    expect(needsAutoPlace(fresh("a", { existing: { folder: "f", filename: "x.tf", label: "a" } }))).toBe(false);
  });

  it("stacks them right of everything drawn, with their own heights, without touching the others", () => {
    const nodes = [fresh("a"), n("moved", "key-vault", { position_x: 500, position_y: 700 }), fresh("b")];
    const out = placeAutoRoots(nodes, layoutDrafts(nodes, hints), { x: 1000, y: 0 });
    const a = out.find((x) => x.id === "a")!, b = out.find((x) => x.id === "b")!;
    expect([a.position_x, a.position_y]).toEqual([1000, 0]);
    expect(b.position_x).toBe(1000);
    expect(b.position_y).toBeGreaterThan(a.position_y + 92); // below a, not on top of it
    expect(out.find((x) => x.id === "moved")).toBe(nodes[1]);
  });

  it("returns the same array when nothing needs a spot", () => {
    const nodes = [n("a", "key-vault")];
    expect(placeAutoRoots(nodes, layoutDrafts(nodes, hints), { x: 0, y: 0 })).toBe(nodes);
  });
});
