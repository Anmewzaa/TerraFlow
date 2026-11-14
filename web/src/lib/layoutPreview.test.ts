import { describe, expect, it } from "vitest";
import { LAYOUT_PRESETS, folderFor, samplePaths, valuesFor } from "./layoutPreview";

describe("layout preview", () => {
  it("shows today's layout by default", () => {
    expect(samplePaths(undefined, undefined, "dev")).toEqual(["vnet/vnet.tf", "storage/storage.tf", "vnet/env/dev.tfvars"]);
  });
  it("puts the environment in the folder when the pattern says so", () => {
    expect(folderFor("{stack}/{env}", "vnet", "dev")).toBe("vnet/dev");
    expect(folderFor("{env}/{stack}", "vnet", "dev")).toBe("dev/vnet");
    expect(samplePaths("{env}", "{dir}/terraform.tfvars", "prod")).toEqual(["prod/vnet.tf", "prod/storage.tf", "prod/terraform.tfvars"]);
  });
  it("uses the project folder for '.' and has no values path for none", () => {
    expect(samplePaths(".", "{dir}/{env}.tfvars", "dev")).toEqual(["vnet.tf", "storage.tf", "dev.tfvars"]);
    expect(valuesFor("{env}", "none", "vnet", "dev")).toBeNull();
  });
  it("has a preview for every preset", () => {
    for (const p of LAYOUT_PRESETS) expect(samplePaths(p.pattern, p.values, "dev").length).toBeGreaterThan(1);
  });
});
