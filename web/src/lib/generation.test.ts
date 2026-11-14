import { describe, expect, it } from "vitest";
import { EMPTY_SETTINGS, environmentsOf, suggestToken, targetFor, tokensIn, type GenerationSettings } from "./generation";

const settings: GenerationSettings = {
  layout: { mode: "catalog", files: { "resource-group": "rg.tf" } },
  variables: [
    { name: "org", values: { dev: "aic", prod: "aic" } },
    { name: "region", values: { dev: "southeastasia", uat: "eastasia" } },
  ],
};

describe("default target of a new resource", () => {
  it("keeps the module's own folder and file without settings", () => {
    expect(targetFor(EMPTY_SETTINGS, "vnet", "network", "vnet.tf")).toEqual({ folder: "network", filename: "vnet.tf" });
    expect(targetFor(EMPTY_SETTINGS, "vnet", "network")).toEqual({ folder: "network" });
  });
  it("applies a file name chosen per kind of resource", () => {
    expect(targetFor(settings, "resource-group", "common", "main.tf")).toEqual({ folder: "common", filename: "rg.tf" });
    expect(targetFor(settings, "vnet", "network", "vnet.tf")).toEqual({ folder: "network", filename: "vnet.tf" });
  });
  it("puts everything in one folder and file in single mode", () => {
    expect(targetFor({ layout: { mode: "single" } }, "vnet", "network", "vnet.tf")).toEqual({ folder: "terraform", filename: "main.tf" });
    expect(targetFor({ layout: { mode: "single", folder: "infra", file: "all.tf" } }, "x", "y")).toEqual({ folder: "infra", filename: "all.tf" });
  });
});

describe("project variables in a form", () => {
  it("finds the declared tokens only", () => {
    expect(tokensIn("rg-{org}-{unknown}", settings)).toEqual(["org"]);
  });
  it("suggests the token for a value that contains a variable's value, never for a text that has one", () => {
    expect(suggestToken("rg-aic-dev", settings)).toEqual({ name: "org", value: "aic", replaced: "rg-{org}-dev" });
    expect(suggestToken("rg-{org}-dev", settings)).toBeNull();
    expect(suggestToken("nothing here", settings)).toBeNull();
    expect(suggestToken("", settings)).toBeNull();
  });
  it("prefers the longest matching value and ignores one-letter values", () => {
    const s: GenerationSettings = { layout: {}, variables: [{ name: "a", values: { dev: "x" } }, { name: "env", values: { dev: "dev" } }, { name: "app", values: { dev: "dev-app" } }] };
    expect(suggestToken("rg-dev-app-1", s)?.name).toBe("app");
    expect(suggestToken("x", s)).toBeNull();
  });
  it("lists the environments that have a value, dev first", () => {
    expect(environmentsOf(settings)).toEqual(["dev", "prod", "uat"]);
    expect(environmentsOf(EMPTY_SETTINGS)).toEqual(["dev"]);
  });
});

import { settingsFromAdvanced } from "../components/GitHubProjectDialog";

describe("advanced choices while creating a project", () => {
  const none = { single: false, folder: "", file: "", provider: "", providerVersion: "", backend: false, vars: [] };
  it("writes nothing when nothing was chosen", () => {
    expect(settingsFromAdvanced(none)).toBeNull();
    expect(settingsFromAdvanced({ ...none, vars: [{ name: "", value: "" }, { name: "org", value: " " }] })).toBeNull();
  });
  it("builds the settings document from the form", () => {
    const s = settingsFromAdvanced({ ...none, single: true, backend: true, provider: "azurerm", providerVersion: " ~> 4.0 ", vars: [{ name: "org", value: " aic " }] });
    expect(s?.layout).toEqual({ mode: "single", folder: "terraform", file: "main.tf" });
    expect(s?.variables).toEqual([{ name: "org", values: { dev: "aic" } }]);
    expect(s?.starters?.map((f) => f.filename)).toEqual(["backend.tf"]);
    expect(s?.providers).toEqual([{ id: "azurerm", version: "~> 4.0" }]);
  });
  it("carries the environments, and puts a typed value in the first one when dev is not among them", () => {
    const s = settingsFromAdvanced({ ...none, environments: " sbx, int , prod", vars: [{ name: "org", value: "aic" }] });
    expect(s?.environments).toEqual(["sbx", "int", "prod"]);
    expect(s?.variables).toEqual([{ name: "org", values: { sbx: "aic" } }]);
    expect(settingsFromAdvanced({ ...none, environments: "dev, prod", vars: [{ name: "org", value: "aic" }] })?.variables?.[0].values).toEqual({ dev: "aic" });
  });
});
