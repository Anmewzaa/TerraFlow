import { describe, expect, it } from "vitest";
import { LEAF_W, layoutDrafts, type HintMap } from "./canvasLayout";
import type { CanvasNode } from "../types/changeset";

const hints: HintMap = new Map([["virtual-network", { container: true }]]);
const node = (id: string, mod: string, extra: Partial<CanvasNode> = {}): CanvasNode => ({
  id, module_name: mod, module_version: "v1", instance_name: id, target_folder: "f", values: {}, position_x: 0, position_y: 0, ...extra,
});

describe("resources placed freely inside a container", () => {
  it("keeps a placed child where it was put and grows the container to hold it", () => {
    const placed = layoutDrafts([node("vnet", "virtual-network"), node("a", "storage", { parent_id: "vnet", free: true, position_x: 200, position_y: 120 })], hints);
    const a = placed.find((p) => p.id === "a")!;
    const vnet = placed.find((p) => p.id === "vnet")!;
    expect([a.x, a.y]).toEqual([200, 120]);
    expect(vnet.w).toBeGreaterThanOrEqual(200 + LEAF_W);
    expect(vnet.minW).toBe(vnet.w);
  });

  it("still stacks a child that was never placed by hand", () => {
    const placed = layoutDrafts([node("vnet", "virtual-network"), node("a", "storage", { parent_id: "vnet" })], hints);
    expect(placed.find((p) => p.id === "a")!.x).toBe(16);
  });

  it("a size dragged larger is kept, and never smaller than the contents", () => {
    const big = layoutDrafts([node("vnet", "virtual-network", { width: 900, height: 700 })], hints)[0]!;
    expect([big.w, big.h]).toEqual([900, 700]);
    expect(big.minW).toBeLessThan(900);
    const small = layoutDrafts([node("vnet", "virtual-network", { width: 10, height: 10 })], hints)[0]!;
    expect(small.w).toBe(small.minW);
  });
});
