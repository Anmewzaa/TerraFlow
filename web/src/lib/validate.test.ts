import { describe, expect, it } from "vitest";
import { defaultValues, nodeIssues } from "./validate";
import type { CanvasNode } from "../types/changeset";
import type { ModuleSummary } from "../types/catalog";

const mod = (extra: Partial<ModuleSummary>): ModuleSummary => ({
  name: "private-endpoint", title: "PE", description: "", icon: "", versions: ["v1"], latest_version: "v1", ...extra,
});
const node = (values: Record<string, unknown>): CanvasNode => ({
  id: "n", module_name: "private-endpoint", module_version: "v1", instance_name: "pe", target_folder: "vnet",
  values, position_x: 0, position_y: 0,
});

describe("nodeIssues", () => {
  const pe = mod({
    required: ["name", "subnet_id", "subresource_names"],
    canvas: { parents: ["subnet"], parent_required: true, parent_ref: { field: "subnet_id", expr: "x" } },
  });

  it("flags a missing parent and empty required inputs", () => {
    const issues = nodeIssues(node({ name: "" }), undefined, pe).map((i) => i.text);
    expect(issues).toContain("Drag it into subnet");
    expect(issues).toContain("name is required");
    expect(issues).toContain("subnet id is required");
    expect(issues).toContain("subresource names is required");
  });

  it("is complete once placed and filled in (parent supplies the subnet)", () => {
    const n = node({ name: "pe-1", subresource_names: ["blob"] });
    expect(nodeIssues(n, "snet", pe)).toEqual([]);
  });

  it("treats whitespace and empty lists as missing, but keeps 0 and false", () => {
    const m = mod({ required: ["a", "b", "c"] });
    expect(nodeIssues(node({ a: "  ", b: [], c: 0 }), undefined, m).map((i) => i.field)).toEqual(["a", "b"]);
  });

  it("has no issues for a module that is unknown to the catalog", () => {
    expect(nodeIssues(node({}), undefined, undefined)).toEqual([]);
  });
});

describe("defaultValues", () => {
  it("copies meaningful defaults and skips empty ones", () => {
    expect(
      defaultValues({
        location: { default: "southeastasia" },
        tags: { default: {} },
        dns: { default: [] },
        size: { default: "Standard_B2s" },
        none: {},
        blank: { default: "" },
        flag: { default: false },
      }),
    ).toEqual({ location: "southeastasia", size: "Standard_B2s", flag: false });
  });
});
