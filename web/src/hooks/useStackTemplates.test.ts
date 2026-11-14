import { describe, expect, it } from "vitest";
import { mergeVariables, parameterChoices } from "./useStackTemplates";
import type { CanvasNode } from "../types/changeset";

const n = (inst: string, values: Record<string, unknown>) => ({ id: inst, instance_name: inst, values }) as unknown as CanvasNode;

describe("stack template helpers", () => {
  it("offers plain single values as parameters, never references, lists or variables", () => {
    const got = parameterChoices([n("sn", { size: "24", count: 3, ref: { $expr: "module.x.id" }, list: ["a"], used: "{org}-x", empty: "" })]);
    expect(got.map((c) => c.field)).toEqual(["size", "count"]);
  });
  it("keeps what the project already says for an environment", () => {
    const got = mergeVariables(
      [{ name: "org", values: { dev: "aic" } }],
      [{ name: "org", values: { dev: "other", prod: "p" } }, { name: "size", values: { dev: "24" } }],
    );
    expect(got).toEqual([{ name: "org", values: { dev: "aic", prod: "p" } }, { name: "size", values: { dev: "24" } }]);
  });
});
