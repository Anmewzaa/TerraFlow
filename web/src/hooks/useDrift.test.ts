import { describe, expect, it } from "vitest";
import { KIND_LABEL } from "./useDrift";

describe("drift labels", () => {
  it("names every kind of item the server can send", () => {
    for (const k of ["unrepresented", "variable_differs", "file_changed", "file_added", "file_removed"] as const) {
      expect(KIND_LABEL[k].length).toBeGreaterThan(3);
    }
  });
});
