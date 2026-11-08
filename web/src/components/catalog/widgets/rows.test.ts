import { describe, expect, it } from "vitest";
import { itemsToList, rowsToObject, toRows, withBlankItem, withBlankRow } from "./rows";

describe("key/value rows", () => {
  it("always ends with one empty row, and never adds a second", () => {
    const rows = withBlankRow(toRows({ owner: "team-a" }));
    expect(rows.map((r) => r.k)).toEqual(["owner", ""]);
    expect(withBlankRow(rows)).toBe(rows);
    expect(withBlankRow([]).length).toBe(1);
  });

  it("typing in the empty row opens the next one", () => {
    const rows = withBlankRow(toRows({}));
    const typed = rows.map((r) => ({ ...r, k: "env" }));
    const next = withBlankRow(typed);
    expect(next.length).toBe(2);
    expect(rowsToObject(next)).toEqual({ env: "" });
  });

  it("drops rows without a key, trims keys, and the last duplicate wins", () => {
    const rows = [
      { id: 1, k: " a ", v: "1" },
      { id: 2, k: "", v: "orphan" },
      { id: 3, k: "a", v: "2" },
    ];
    expect(rowsToObject(rows)).toEqual({ a: "2" });
  });

  it("non-string values come back as text", () => {
    expect(toRows({ n: 3 as unknown as string })[0].v).toBe("3");
  });
});

describe("list rows", () => {
  it("keeps one empty item at the end and drops empties on the way out", () => {
    const rows = withBlankItem([{ id: 1, text: "10.0.0.0/16" }]);
    expect(rows.length).toBe(2);
    expect(itemsToList([...rows.slice(0, 1), { id: 9, text: " 10.1.0.0/16 " }, { id: 10, text: "" }])).toEqual([
      "10.0.0.0/16",
      "10.1.0.0/16",
    ]);
  });
});
