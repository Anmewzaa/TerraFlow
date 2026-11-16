import { describe, expect, it } from "vitest";
import { diffCanvases, type ResourceDiff } from "./useEnvDiff";

const node = (inst: string, values: Record<string, unknown> = {}) => ({ module_name: "subnet", instance_name: inst, target_folder: "vnet", values });

describe("diffCanvases", () => {
  it("pairs by folder, module and label, and reports what differs", () => {
    const d = diffCanvases("dev", "prod", [node("a", { size: "S", keep: 1 }), node("gone")], [node("a", { size: "L", keep: 1, extra: true }), node("new")]);
    const by: Record<string, ResourceDiff> = {};
    for (const r of d.resources) by[r.label ?? ""] = r;
    expect(by["a"]?.status).toBe("differs");
    expect((by["a"]?.args ?? []).map((x) => [x.name, x.status])).toEqual([
      ["extra", "only_to"],
      ["size", "differs"],
    ]);
    expect(by["gone"]?.status).toBe("only_from");
    expect(by["new"]?.status).toBe("only_to");
    expect(d.summary).toEqual({ same: 0, differs: 1, unknown: 0, only_from: 1, only_to: 1 });
  });

  it("calls identical resources the same, and empty canvases an empty comparison", () => {
    expect(diffCanvases("a", "b", [node("x", { n: 1 })], [node("x", { n: 1 })]).summary.same).toBe(1);
    expect(diffCanvases("a", "b", [], []).resources).toEqual([]);
  });
});
