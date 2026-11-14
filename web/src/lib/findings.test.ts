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
