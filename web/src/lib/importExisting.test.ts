import { describe, expect, it } from "vitest";
import { resolveCatalogVersion, toCanvasNode, versionFromSource } from "./importExisting";

const imp = {
  id: "network.module.vnet",
  module_name: "virtual-network",
  instance_name: "vnet",
  target_folder: "network",
  source: "git::https://x/_git/m//virtual-network?ref=v1.2.0",
  filename: "main.tf",
  values: { name: { $expr: "local.vnet_name" } },
  existing: { folder: "network", filename: "main.tf", label: "vnet" },
};

describe("importExisting", () => {
  it("reads the pinned version from the source", () => {
    expect(versionFromSource(imp.source)).toBe("v1.2.0");
    expect(versionFromSource("./local")).toBe("");
    expect(versionFromSource(undefined)).toBe("");
  });

  it("keeps the repository location and expressions", () => {
    const n = toCanvasNode(imp, 5);
    expect(n.existing).toEqual(imp.existing);
    expect(n.module_version).toBe("v1.2.0");
    expect(n.values.name).toEqual({ $expr: "local.vnet_name" });
    expect(n.position_x).toBe(40 + 280);
    expect(n.position_y).toBe(40 + 180);
  });

  it("maps a pinned version to one the catalog has", () => {
    const cat = { versions: ["v1", "v2"], latest_version: "v2" };
    expect(resolveCatalogVersion("v1", cat)).toBe("v1");
    expect(resolveCatalogVersion("v1.2.0", cat)).toBe("v1");
    expect(resolveCatalogVersion("v9.0.0", cat)).toBe("v2");
    expect(resolveCatalogVersion("", cat)).toBe("v2");
    expect(resolveCatalogVersion("v1.2.0", undefined)).toBe("v1.2.0");
  });

  it("uses the catalog version on the node", () => {
    const n = toCanvasNode(imp, 0, { versions: ["v1"], latest_version: "v1" });
    expect(n.module_version).toBe("v1");
    expect(n.source).toBe(imp.source);
  });
});
