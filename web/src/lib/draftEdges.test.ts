import { describe, expect, it } from "vitest";
import type { CanvasNode } from "../types/changeset";
import { referenceEdges } from "./draftEdges";

const node = (id: string, instance: string, values: Record<string, unknown>, folder = "network"): CanvasNode => ({
  id, module_name: "m", module_version: "v1", instance_name: instance, target_folder: folder,
  values, position_x: 0, position_y: 0,
});

describe("referenceEdges", () => {
  const repo = new Set(["network.module.rg_net", "network.module.vnet"]);

  it("links a draft to a repository resource it references", () => {
    const d = node("new1", "nsg", { resource_group_name: { $expr: "module.rg_net.name" } });
    expect(referenceEdges([d], repo)).toEqual([
      { id: "ref:network.module.rg_net->new1", from: "network.module.rg_net", to: "new1" },
    ]);
  });

  it("links draft to draft by instance name, and finds refs in nested values", () => {
    const a = node("a", "vnet2", {});
    const b = node("b", "nsg", {
      subnet_ids: { w: { $expr: "module.vnet2.subnet_ids[0]" } },
      list: ["module.vnet2.id"],
    });
    expect(referenceEdges([a, b], repo)).toEqual([{ id: "ref:a->b", from: "a", to: "b" }]);
  });

  it("ignores unknown names, other folders and self references", () => {
    const d = node("d", "self", { a: "module.nope.id", b: "module.self.id" });
    const other = node("o", "x", { a: "module.rg_net.id" }, "workload");
    expect(referenceEdges([d, other], repo)).toEqual([]);
  });

  it("an imported node keeps its repository id", () => {
    const imp = node("network.module.vnet", "vnet", { rg: { $expr: "module.rg_net.name" } });
    expect(referenceEdges([imp], repo).map((e) => e.id)).toEqual([
      "ref:network.module.rg_net->network.module.vnet",
    ]);
  });
});

describe("link edges", () => {
  it("marks a reference in a link field, and leaves other references plain", () => {
    const kv = { id: "kv", module_name: "key-vault", instance_name: "kv", target_folder: "net", values: {} } as unknown as CanvasNode;
    const pe = {
      id: "pe", module_name: "private-endpoint", instance_name: "pe", target_folder: "net",
      values: { target_resource_id: { $expr: "module.kv.id" }, note: { $expr: "module.rg.name" } },
    } as unknown as CanvasNode;
    const rg = { id: "rg", module_name: "resource-group", instance_name: "rg", target_folder: "net", values: {} } as unknown as CanvasNode;
    const edges = referenceEdges([kv, pe, rg], new Set(), new Map([["private-endpoint", { target_resource_id: "private_link" }]]));
    expect(edges.find((e) => e.from === "kv")?.link).toBe("private_link");
    expect(edges.find((e) => e.from === "rg")?.link).toBeUndefined();
  });
});
