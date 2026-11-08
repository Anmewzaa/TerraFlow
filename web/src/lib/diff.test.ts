import { describe, expect, it } from "vitest";
import { diffFiles, diffLines, diffWithRepo } from "./diff";

describe("diffLines", () => {
  it("marks every line added when there is no base", () => {
    const d = diffLines("", "a\nb\n");
    expect(d.kinds).toEqual(["add", "add"]);
    expect(d.added).toBe(2);
  });

  it("marks a changed line as modified (and remembers the old text)", () => {
    const d = diffLines("a\nname = \"x\"\nc\n", "a\nname = \"y\"\nc\n");
    expect(d.kinds).toEqual(["same", "mod", "same"]);
    expect(d.previous[1]).toBe('name = "x"');
    expect(d.text).toBe('a\nname = "y"\nc\n');
    expect([d.added, d.modified, d.removed]).toEqual([0, 1, 0]);
  });

  it("splices removed lines back in and keeps added ones", () => {
    const d = diffLines("a\nb\nc\n", "a\nc\nd\n");
    expect(d.text).toBe("a\nb\nc\nd\n");
    expect(d.kinds).toEqual(["same", "del", "same", "add"]);
  });

  it("pairs unequal runs: extra removals stay red, extra additions stay green", () => {
    const d = diffLines("x\ny\nz\n", "p\n");
    expect(d.kinds).toEqual(["mod", "del", "del"]);
    const e = diffLines("x\n", "p\nq\nr\n");
    expect(e.kinds).toEqual(["mod", "add", "add"]);
  });

  it("is a no-op for identical content", () => {
    const d = diffLines("a\nb\n", "a\r\nb\r\n");
    expect(d.kinds.every((k) => k === "same")).toBe(true);
  });
});

describe("diffFiles", () => {
  const f = (folder: string, filename: string, content: string) => ({ folder, filename, content });
  it("classifies added, modified, removed and unchanged files", () => {
    const base = [f("a", "main.tf", "1\n"), f("b", "main.tf", "x\n"), f("c", "main.tf", "same\n")];
    const cur = [f("a", "main.tf", "2\n"), f("c", "main.tf", "same\n"), f("d", "main.tf", "new\n")];
    const r = Object.fromEntries(diffFiles(base, cur).map((d) => [d.path, d.status]));
    expect(r).toEqual({
      "a/main.tf": "modified",
      "b/main.tf": "removed",
      "c/main.tf": "unchanged",
      "d/main.tf": "added",
    });
  });
});

describe("diffWithRepo", () => {
  const f = (folder: string, filename: string, content: string) => ({ folder, filename, content });
  it("shows existing repo files unchanged and drafts as additions", () => {
    const repo = [f("vnet", "vnet.tf", "a\n"), f("kv", "kv.tf", "b\n")];
    const cur = [f("vnet", "main.tf", "new\n")];
    const r = Object.fromEntries(diffWithRepo(repo, [], cur).map((d) => [d.path, d.status]));
    expect(r).toEqual({ "kv/kv.tf": "unchanged", "vnet/vnet.tf": "unchanged", "vnet/main.tf": "added" });
  });

  it("diffs a generated file against the repo file at the same path", () => {
    const d = diffWithRepo([f("a", "main.tf", "x\n")], [], [f("a", "main.tf", "y\n")]);
    expect(d[0].status).toBe("modified");
    expect(d[0].diff.kinds).toEqual(["mod"]);
  });

  it("prefers the revision base over the repo file", () => {
    const d = diffWithRepo([f("a", "main.tf", "old\n")], [f("a", "main.tf", "mid\n")], [f("a", "main.tf", "mid\n")]);
    expect(d[0].status).toBe("unchanged");
  });

  it("marks files only in the base as removed", () => {
    const d = diffWithRepo([], [f("a", "main.tf", "x\n")], []);
    expect(d[0].status).toBe("removed");
  });
});
