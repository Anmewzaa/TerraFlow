import { describe, expect, it } from "vitest";
import { FILENAME_RE, FOLDER_RE } from "./FileTargetFields";

describe("output file validation (mirrors the server rules)", () => {
  it("accepts the team's file names", () => {
    for (const ok of ["rg.tf", "kv.tf", "main.tf", "vnet-main.tf", "a_b.c.tf"]) expect(FILENAME_RE.test(ok)).toBe(true);
  });
  it("rejects paths, wrong extensions and odd characters", () => {
    for (const bad of ["../x.tf", "a/b.tf", "x.txt", "main", ".tf", "a b.tf", "x.tf.sh", ""]) expect(FILENAME_RE.test(bad)).toBe(false);
  });
  it("folders are single plain segments", () => {
    expect(FOLDER_RE.test("keyvault")).toBe(true);
    expect(FOLDER_RE.test("a/b")).toBe(false);
    expect(FOLDER_RE.test("..")).toBe(false);
  });
});
