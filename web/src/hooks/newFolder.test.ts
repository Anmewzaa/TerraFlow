import { describe, expect, it } from "vitest";
import { NEW_FOLDER_RE } from "./useGitHub";

describe("new folder path (mirrors the server's safe-path rules)", () => {
  it("accepts plain nested folders", () => {
    for (const ok of ["iac", "terraform/dev", "a/b/c/d", "env-1/prod_2", "v1.2"]) expect(NEW_FOLDER_RE.test(ok)).toBe(true);
  });
  it("rejects traversal, absolute, backslash, spaces and too deep", () => {
    for (const bad of ["../x", "/abs", "a//b", "a\b", "a b", ".hidden", "a/b/c/d/e", "", "a/"]) expect(NEW_FOLDER_RE.test(bad)).toBe(false);
  });
});
