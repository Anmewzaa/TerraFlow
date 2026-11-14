import { describe, expect, it } from "vitest";
import { buildProjectHash, parseProjectHash } from "./projectHash";

describe("project hash", () => {
  it("reads a project with and without a tab", () => {
    expect(parseProjectHash("#/p/abc")).toEqual({ projectId: "abc", tab: "project" });
    expect(parseProjectHash("#/p/abc/settings")).toEqual({ projectId: "abc", tab: "settings" });
  });
  it("ignores a tab it does not know and a broken id", () => {
    expect(parseProjectHash("#/p/abc/other").projectId).toBeNull();
    expect(parseProjectHash("#/p/%E0%A4%A").projectId).toBeNull();
    expect(parseProjectHash("").projectId).toBeNull();
  });
  it("builds what it reads", () => {
    for (const h of [buildProjectHash("a b"), buildProjectHash("a b", "settings")]) {
      expect(buildProjectHash(parseProjectHash("#" + h).projectId, parseProjectHash("#" + h).tab)).toBe(h);
    }
    expect(buildProjectHash(null, "settings")).toBe("");
  });
});
