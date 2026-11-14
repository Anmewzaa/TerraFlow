import { describe, expect, it } from "vitest";
import { currentLink, linkTargets, linkValue } from "./links";
import type { ModuleSummary } from "../types/catalog";
import type { CanvasNode } from "../types/changeset";

const mods = [
  { name: "key-vault", offers: [{ kind: "private_link", output: "id", subresource: "vault" }] },
  { name: "resource-group" },
] as unknown as ModuleSummary[];

const draft = (id: string, module: string, name: string): CanvasNode =>
  ({ id, module_name: module, instance_name: id, target_folder: "vnet", values: { name } }) as unknown as CanvasNode;

describe("links", () => {
  it("offers only the resources that advertise the kind", () => {
    const t = linkTargets("private_link", mods, null, [draft("kv", "key-vault", "kv-a"), draft("rg", "resource-group", "rg-a")]);
    expect(t.map((x) => x.label)).toEqual(["kv"]);
  });
  it("finds repository blocks too, by their label", () => {
    const graph = { folders: [{ name: "vault", nodes: [{ kind: "module", type: "key-vault", label: "kv_main", name: "kv-x" }] }] } as never;
    expect(linkTargets("private_link", mods, graph, [])[0]).toMatchObject({ label: "kv_main", folder: "vault", draft: false });
  });
  it("writes the reference and fills the sub-resource", () => {
    const [t] = linkTargets("private_link", mods, null, [draft("kv", "key-vault", "kv-a")]);
    expect(linkValue(t, false, "subresource_names")).toEqual({ value: { $expr: "module.kv.id" }, siblings: { subresource_names: ["vault"] } });
    expect(linkValue(t, true, undefined)).toEqual({ value: [{ $expr: "module.kv.id" }], siblings: undefined });
  });
  it("reads back what a field holds", () => {
    expect(currentLink({ $expr: "module.kv.id" })).toBe("module.kv.id");
    expect(currentLink([{ $expr: "module.z.id" }])).toBe("module.z.id");
    expect(currentLink("plain")).toBe("");
  });
});
