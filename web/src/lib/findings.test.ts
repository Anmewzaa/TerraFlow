import { describe, expect, it } from "vitest";
import { describeFindings, type Finding } from "./findings";

const f = (message: string, field?: string, severity: Finding["severity"] = "error"): Finding => ({ severity, rule: "x", message, field });

describe("describeFindings", () => {
  it("falls back to the reason when there are no errors", () => {
    expect(describeFindings("nope", [])).toBe("nope");
    expect(describeFindings(undefined, undefined)).toBe("");
    expect(describeFindings("nope", [f("only a warning", undefined, "warning")])).toBe("nope");
  });
  it("lists the first problems and counts the rest", () => {
    const list = [f("is required", "resource_group_name"), f("b"), f("c"), f("d"), f("e")];
    expect(describeFindings("Fix these", list)).toBe("Fix these: resource group name: is required; b; c; d (and 1 more)");
  });
});

import { serverProblems } from "./findings";

describe("serverProblems", () => {
  const fs: Finding[] = [
    { severity: "error", node: "a", rule: "required", message: "name is required" },
    { severity: "warning", node: "a", rule: "policy", message: "name must start with rg-" },
    { severity: "warning", node: "a", rule: "schema", message: "a plain warning" },
    { severity: "warning", node: "b", rule: "policy", message: "other node" },
  ];
  it("keeps errors and policy warnings apart, for one node", () => {
    expect(serverProblems(fs, "a")).toEqual({ errors: ["name is required"], warnings: ["name must start with rg-"] });
  });
  it("does not repeat what the panel already says", () => {
    expect(serverProblems(fs, "a", ["Name is required"]).errors).toEqual([]);
  });
});
