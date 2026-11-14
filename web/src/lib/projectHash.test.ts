import { describe, expect, it } from "vitest";
import { buildProjectHash, parseProjectHash } from "./projectHash";

describe("project hash", () => {
  it("reads the canvas, the rail views and the two tabs of the project page", () => {
    expect(parseProjectHash("#/p/abc")).toEqual({ projectId: "abc", view: "canvas", tab: "project" });
    expect(parseProjectHash("#/p/abc/compare")).toEqual({ projectId: "abc", view: "compare", tab: "project" });
    expect(parseProjectHash("#/p/abc/team")).toEqual({ projectId: "abc", view: "project", tab: "project" });
    expect(parseProjectHash("#/p/abc/settings")).toEqual({ projectId: "abc", view: "project", tab: "settings" });
  });
  it("ignores a screen it does not know and a broken id", () => {
    expect(parseProjectHash("#/p/abc/other").projectId).toBeNull();
    expect(parseProjectHash("#/p/%E0%A4%A").projectId).toBeNull();
    expect(parseProjectHash("").projectId).toBeNull();
  });
  it("builds what it reads, for every screen", () => {
    for (const h of ["/p/a%20b", "/p/a%20b/changesets", "/p/a%20b/review", "/p/a%20b/hcl", "/p/a%20b/compare", "/p/a%20b/team", "/p/a%20b/settings"]) {
      const p = parseProjectHash("#" + h);
      expect(buildProjectHash(p.projectId, p.view, p.tab)).toBe(h);
    }
    expect(buildProjectHash(null, "compare")).toBe("");
  });
});
