import { describe, expect, it } from "vitest";
import { displayName, parentsFirst, valueText } from "./valueText";

describe("valueText", () => {
  it("never returns an object", () => {
    expect(valueText("rg-a")).toBe("rg-a");
    expect(valueText(3)).toBe("3");
    expect(valueText({ $expr: "local.rg_names.network" })).toBe("local.rg_names.network");
    expect(valueText({ other: 1 })).toBe("");
    expect(valueText(undefined)).toBe("");
  });

  it("falls back to the instance name", () => {
    expect(displayName({ name: { $expr: "var.n" } }, "rg")).toBe("var.n");
    expect(displayName({}, "rg")).toBe("rg");
    expect(displayName(undefined, "rg")).toBe("rg");
  });
});

describe("parentsFirst", () => {
  it("moves parents before children and keeps sibling order", () => {
    const out = parentsFirst([
      { id: "child1", parentId: "p" },
      { id: "x" },
      { id: "child2", parentId: "p" },
      { id: "p" },
      { id: "grand", parentId: "child1" },
    ]);
    expect(out.map((n) => n.id)).toEqual(["x", "p", "child1", "child2", "grand"]);
  });

  it("does not loop on a cycle", () => {
    expect(parentsFirst([{ id: "a", parentId: "b" }, { id: "b", parentId: "a" }]).length).toBe(2);
  });
});
