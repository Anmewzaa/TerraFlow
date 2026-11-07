import { describe, expect, it } from "vitest";
import { toneForStatus } from "./design-system/components";

describe("toneForStatus", () => {
  it("maps lifecycle words to tones", () => {
    expect(toneForStatus("approved")).toBe("good");
    expect(toneForStatus("rejected")).toBe("bad");
    expect(toneForStatus("pending review")).toBe("warn");
    expect(toneForStatus("external")).toBe("info");
    expect(toneForStatus("whatever")).toBe("neutral");
  });
});
