import { describe, expect, it } from "vitest";
import { compatible, fitToProject, providerOfType } from "./provider";

describe("provider version compatibility", () => {
  it("accepts a resource whose range contains the project's version", () => {
    expect(compatible("~> 4.0")).toBe(true);
    expect(compatible("~> 4.0", "3.0")).toBe(true);
    expect(compatible("~> 4.0", "4.0", "4.0")).toBe(true);
    expect(compatible("= 4.1.0", "4.1")).toBe(true);
  });
  it("rejects one added later or removed earlier", () => {
    expect(compatible("~> 4.0", "4.1")).toBe(false);
    expect(compatible("~> 3.100", "", "3.50")).toBe(false);
  });
  it("hides nothing when the project has no readable version", () => {
    expect(compatible(undefined, "9.9")).toBe(true);
    expect(compatible("latest", "9.9")).toBe(true);
  });
  it("names the provider from the resource type", () => {
    expect(providerOfType("azurerm_resource_group")).toBe("azurerm");
    expect(providerOfType("nounderscore")).toBe("");
  });
});

describe("raw resources that fit a project", () => {
  const rg = { resource_type: "azurerm_resource_group", provider: "azurerm" };
  const newer = { resource_type: "azurerm_new_thing", provider: "azurerm", min_version: "4.2" };
  const bucket = { resource_type: "aws_s3_bucket", provider: "aws" };
  const legacy = { resource_type: "azurerm_old_thing" }; // provider taken from the type
  const all = [rg, newer, bucket, legacy];

  it("shows everything to a project that chose no provider", () => {
    expect(fitToProject(all, []).shown).toHaveLength(4);
    expect(fitToProject(all, undefined).hidden).toHaveLength(0);
  });
  it("keeps only the chosen provider at a version that fits, and says what was hidden", () => {
    const r = fitToProject(all, [{ id: "azurerm", version: "~> 4.0" }]);
    expect(r.shown.map((x) => x.resource_type)).toEqual(["azurerm_resource_group", "azurerm_old_thing"]);
    expect(r.hidden.map((x) => x.resource_type)).toEqual(["azurerm_new_thing", "aws_s3_bucket"]);
  });
  it("a project using two providers sees both", () => {
    const r = fitToProject(all, [{ id: "azurerm", version: "~> 4.5" }, { id: "aws" }]);
    expect(r.shown.map((x) => x.resource_type)).toEqual(["azurerm_resource_group", "azurerm_new_thing", "aws_s3_bucket", "azurerm_old_thing"]);
  });
});
