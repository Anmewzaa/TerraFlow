import { describe, expect, it } from "vitest";
import { serverIssueTexts } from "./useValidation";
import type { Finding } from "../lib/findings";

const f = (node: string, message: string, severity: Finding["severity"] = "error"): Finding => ({ node, message, severity, rule: "x" });

describe("serverIssueTexts", () => {
  const findings = [f("a", "name is required"), f("a", "lock level must be one of CanNotDelete, ReadOnly"), f("b", "other node"), f("a", "just a warning", "warning")];
  it("keeps only this node's errors", () => {
    expect(serverIssueTexts(findings, "a", [])).toEqual(["name is required", "lock level must be one of CanNotDelete, ReadOnly"]);
  });
  it("does not repeat what the panel already says, ignoring case", () => {
    expect(serverIssueTexts(findings, "a", ["Name is required"])).toEqual(["lock level must be one of CanNotDelete, ReadOnly"]);
  });
  it("is empty for a node with no problems", () => {
    expect(serverIssueTexts(findings, "zzz", [])).toEqual([]);
  });
});
