import { describe, expect, it } from "vitest";
import { AZURE_REGIONS, choicesOf } from "./options";
import { getAzureIcon } from "../components/nodes/domainStyles";

describe("choicesOf", () => {
  it("offers the choices a schema lists, with labels", () => {
    const set = choicesOf("tier", { type: "string", "x-tf-ui": { options: ["a", { value: "b", label: "Bee" }, 3 as never, ""] } });
    expect(set?.choices).toEqual([{ value: "a", label: "a" }, { value: "b", label: "Bee" }]);
    expect(set?.allowCustom).toBe(false);
  });
  it("lets the schema allow a typed value", () => {
    expect(choicesOf("tier", { type: "string", "x-tf-ui": { options: ["a"], allow_custom: true } })?.allowCustom).toBe(true);
  });
  it("uses a named shared list, which allows other values unless the schema says not", () => {
    expect(choicesOf("where", { type: "string", "x-tf-ui": { options_from: "azure-regions" } })).toEqual({ choices: AZURE_REGIONS, allowCustom: true });
    expect(choicesOf("where", { type: "string", "x-tf-ui": { options_from: "azure-regions", allow_custom: false } })?.allowCustom).toBe(false);
    expect(choicesOf("where", { type: "string", "x-tf-ui": { options_from: "no-such-list" } })).toBeNull();
  });
  it("gives an input named location the regions unless the schema asks for something else", () => {
    expect(choicesOf("location", { type: "string" })?.choices).toBe(AZURE_REGIONS);
    expect(choicesOf("location", { type: "string", "x-tf-ui": { widget: "text" } })).toBeNull();
    expect(choicesOf("location", { type: "string", enum: ["a"] })).toBeNull(); // the enum dropdown handles it
    expect(choicesOf("location", { type: "string", "x-tf-ui": { options: ["x"] } })?.choices).toEqual([{ value: "x", label: "x" }]);
    expect(choicesOf("name", { type: "string" })).toBeNull();
  });
  it("lists every region once", () => {
    const values = AZURE_REGIONS.map((r) => r.value);
    expect(new Set(values).size).toBe(values.length);
  });
});

describe("getAzureIcon", () => {
  it("finds an icon by module name, resource type or the icon name in the schema", () => {
    expect(getAzureIcon("subnet")).toBe("/azure-icons/subnet.svg");
    expect(getAzureIcon("virtual_network")).toBe("/azure-icons/virtual-network.svg");
    expect(getAzureIcon("something-new", "KeyVault")).toBe("/azure-icons/key-vault.svg");
  });
  it("takes a bundled path or a small SVG written in the schema, nothing else", () => {
    expect(getAzureIcon("x", "/azure-icons/custom.svg")).toBe("/azure-icons/custom.svg");
    expect(getAzureIcon("x", "data:image/svg+xml;base64,AAAA")).toBe("data:image/svg+xml;base64,AAAA");
    expect(getAzureIcon("x", "https://evil.example/a.svg")).toBeNull();
    expect(getAzureIcon("x", "javascript:alert(1)")).toBeNull();
    expect(getAzureIcon("x", "/azure-icons/../../secret.svg")).toBeNull();
    expect(getAzureIcon("x", "data:image/svg+xml," + "a".repeat(20000))).toBeNull();
  });
  it("is null for a resource without an icon, never a broken path", () => {
    expect(getAzureIcon("route-table", "RouteTable")).toBeNull();
    expect(getAzureIcon("_comment")).toBeNull();
  });
});
