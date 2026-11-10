import { describe, expect, it } from "vitest";
import type { ModuleSummary } from "../types/catalog";
import { groupByCategory, matches, paletteModules, paletteRaw, usedCategories } from "./palette";

const cats = [
  { id: "compute", label: "Compute" },
  { id: "network", label: "Network" },
  { id: "other", label: "Other" },
];
const mod = (name: string, over: Partial<ModuleSummary> = {}): ModuleSummary => ({
  name, title: name.replace(/-/g, " "), description: "", icon: "", versions: ["v1"], latest_version: "v1", ...over,
});
const modules = [
  mod("virtual-network", { category: "network", provider: "azurerm", description: "VNet with subnets" }),
  mod("linux-vm", { category: "compute", provider: "azurerm" }),
  mod("s3-bucket", { category: "storage", provider: "aws" }),
  mod("generic-thing"), // no provider, no category
];

describe("palette search", () => {
  it("needs every word, in any order and case", () => {
    expect(matches("net vnet", "Virtual Network", "vnet")).toBe(true);
    expect(matches("VNET net", "Virtual Network", "vnet")).toBe(true);
    expect(matches("net bucket", "Virtual Network", "vnet")).toBe(false);
    expect(matches("   ", "anything")).toBe(true);
  });
});

describe("modules in the palette", () => {
  it("hides a module written for a provider the project does not use, and counts it", () => {
    const r = paletteModules(modules, { query: "", category: "", providers: [{ id: "azurerm" }] });
    expect(r.shown.map((m) => m.name)).toEqual(["virtual-network", "linux-vm", "generic-thing"]);
    expect(r.hiddenByProvider).toBe(1);
  });
  it("shows everything to a project that chose no provider", () => {
    expect(paletteModules(modules, { query: "", category: "", providers: [] }).shown).toHaveLength(4);
  });
  it("filters by category and by search together", () => {
    expect(paletteModules(modules, { query: "", category: "network", providers: [] }).shown.map((m) => m.name)).toEqual(["virtual-network"]);
    expect(paletteModules(modules, { query: "vnet", category: "", providers: [] }).shown.map((m) => m.name)).toEqual(["virtual-network"]);
    expect(paletteModules(modules, { query: "vnet", category: "compute", providers: [] }).shown).toHaveLength(0);
    // a module without a category is in "other"
    expect(paletteModules(modules, { query: "", category: "other", providers: [] }).shown.map((m) => m.name)).toEqual(["generic-thing"]);
  });
});

describe("raw resources in the palette", () => {
  const raws = [
    { title: "Resource group", resource_type: "azurerm_resource_group", category: "other" },
    { title: "Public IP", resource_type: "azurerm_public_ip", category: "network", min_version: "4.5" },
    { title: "Bucket", resource_type: "aws_s3_bucket", category: "storage" },
  ];
  it("keeps the project's provider and version, then applies category and search", () => {
    const f = { query: "", category: "", providers: [{ id: "azurerm", version: "~> 4.0" }] };
    const r = paletteRaw(raws, f);
    expect(r.shown.map((t) => t.title)).toEqual(["Resource group"]);
    expect(r.hidden).toBe(2);
    expect(paletteRaw(raws, { ...f, query: "group" }).shown).toHaveLength(1);
    expect(paletteRaw(raws, { ...f, category: "network" }).shown).toHaveLength(0);
  });
});

describe("grouping", () => {
  it("follows the server's category order and sends unknown categories to other", () => {
    const g = groupByCategory(modules, (m) => m.category ?? "other", cats);
    expect(g.map((x) => x.category.id)).toEqual(["compute", "network", "other"]);
    expect(g[2].items.map((m) => m.name)).toEqual(["s3-bucket", "generic-thing"]); // storage is not in this order list
  });
  it("lists only categories that have something", () => {
    expect(usedCategories([modules[0]], [{ category: "compute" }], cats).map((c) => c.id)).toEqual(["compute", "network"]);
  });
});
