import { describe, expect, it } from "vitest";
import { newRawResource, parseValues, rawIssues } from "./rawResource";

describe("rawResource", () => {
  it("a new node is flagged until it has a type and a name", () => {
    const n = newRawResource("network", 0);
    expect(n.kind).toBe("resource");
    expect(rawIssues(n).map((i) => i.field)).toEqual(["resource_type", "instance_name"]);
    expect(rawIssues({ ...n, resource_type: "azurerm_resource_group", instance_name: "rg" })).toEqual([]);
  });

  it("starts from an admin template: its type and a copy of its default values", () => {
    const defaults = { location: "southeastasia", tags: { env: "dev" } };
    const n = newRawResource("network", 1, { resource_type: "azurerm_resource_group", default_values: defaults });
    expect(n.resource_type).toBe("azurerm_resource_group");
    expect(n.values).toEqual(defaults);
    (n.values.tags as Record<string, string>).env = "prod";
    expect((defaults.tags as Record<string, string>).env).toBe("dev"); // not shared
    expect(rawIssues(n).map((i) => i.field)).toEqual(["instance_name"]);
  });

  it("rejects a module-style type and a name with spaces", () => {
    const n = { ...newRawResource("network", 0), resource_type: "ResourceGroup", instance_name: "my rg" };
    expect(rawIssues(n).length).toBe(2);
  });

  it("parses arguments as a JSON object only", () => {
    expect(parseValues("")).toEqual({ values: {} });
    expect(parseValues('{"name":"x","tags":{"$expr":"local.t"}}')).toEqual({
      values: { name: "x", tags: { $expr: "local.t" } },
    });
    expect("error" in parseValues("[1]")).toBe(true);
    expect("error" in parseValues("{bad")).toBe(true);
  });
});
