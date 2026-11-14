import { describe, expect, it } from "vitest";
import { filterDiff, showValue, type ResourceDiff } from "./useEnvDiff";

const r = (id: string, stack: string, status: ResourceDiff["status"]): ResourceDiff => ({ id, stack, kind: "module", type: "t", status });

describe("compare helpers", () => {
  const rows = [r("a", "net", "same"), r("b", "net", "differs"), r("c", "app", "only_to")];
  it("keeps only differences when asked", () => {
    expect(filterDiff(rows, true, "").map((x) => x.id)).toEqual(["b", "c"]);
  });
  it("filters by stack", () => {
    expect(filterDiff(rows, false, "app").map((x) => x.id)).toEqual(["c"]);
  });
  it("says unknown for an unresolved value, never blank", () => {
    expect(showValue({ $unknown: true })).toBe("unknown");
    expect(showValue(undefined)).toBe("—");
    expect(showValue(["a"])).toBe('["a"]');
  });
});
