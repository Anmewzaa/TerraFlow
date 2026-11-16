import { describe, expect, it } from "vitest";
import { cidrText, contains, freeAddresses, infoOf, nextFree, overlaps, parseCidr, problemsOf, type Cidr } from "./cidr";

const c = (s: string): Cidr => {
  const p = parseCidr(s);
  if (!p) throw new Error(s);
  return p;
};

describe("parseCidr", () => {
  it("reads IPv4 blocks and refuses everything else", () => {
    expect(cidrText(c("10.0.1.0/24"))).toBe("10.0.1.0/24");
    for (const bad of ["", "10.0.1.0", "10.0.1/24", "10.0.1.0/33", "300.0.0.1/24", "10.0.01.0/24", "10.0.1.0/-1", "a.b.c.d/8", "10.0.0.0/8/8"]) {
      expect(parseCidr(bad)).toBeNull();
    }
  });
});

describe("block maths", () => {
  it("counts addresses the way Azure does", () => {
    const i = infoOf(c("10.0.1.0/24"));
    expect(i).toMatchObject({ total: 256, usable: 251, first: "10.0.1.0", last: "10.0.1.255", firstUsable: "10.0.1.4", lastUsable: "10.0.1.254" });
    expect(infoOf(c("10.0.0.0/29")).usable).toBe(3);
    expect(infoOf(c("10.0.0.0/32")).usable).toBe(0);
    expect(infoOf(c("0.0.0.0/0")).total).toBe(2 ** 32);
  });
  it("tells overlap and containment", () => {
    expect(overlaps(c("10.0.1.0/24"), c("10.0.1.128/25"))).toBe(true);
    expect(overlaps(c("10.0.1.0/24"), c("10.0.2.0/24"))).toBe(false);
    expect(contains(c("10.0.0.0/16"), c("10.0.255.0/24"))).toBe(true);
    expect(contains(c("10.0.0.0/16"), c("10.1.0.0/24"))).toBe(false);
    expect(contains(c("10.0.1.0/24"), c("10.0.0.0/16"))).toBe(false);
    expect(overlaps(c("255.255.255.0/24"), c("255.255.255.128/25"))).toBe(true); // top of the range
  });
});

describe("problemsOf", () => {
  const space = [c("10.0.0.0/16")];
  const used = [{ label: "snet-a", block: c("10.0.1.0/24") }];
  it("says nothing about an empty or IPv6 entry", () => {
    expect(problemsOf("", space, used)).toEqual([]);
    expect(problemsOf("fd00::/64", space, used)).toEqual([]);
  });
  it("reports syntax, host bits, outside and overlap", () => {
    expect(problemsOf("10.0.1", space, used)[0].kind).toBe("syntax");
    const hb = problemsOf("10.0.2.5/24", space, used);
    expect(hb[0].kind).toBe("host-bits");
    expect(hb[0].text).toContain("10.0.2.0/24");
    expect(problemsOf("10.1.0.0/24", space, used).map((p) => p.kind)).toEqual(["outside"]);
    expect(problemsOf("10.0.1.128/25", space, used).map((p) => p.kind)).toEqual(["overlap"]);
    expect(problemsOf("10.0.2.0/24", space, used)).toEqual([]);
  });
  it("knows Azure's smallest subnet", () => {
    expect(problemsOf("10.0.2.0/30", space, [], { minBits: 29 })[0].kind).toBe("small");
    expect(problemsOf("10.0.2.0/29", space, [], { minBits: 29 })).toEqual([]);
  });
  it("does not judge containment without a known space", () => {
    expect(problemsOf("172.16.0.0/24", [], used)).toEqual([]);
  });
});

describe("nextFree", () => {
  const space = [c("10.0.0.0/16")];
  it("takes the first aligned block that touches nothing", () => {
    expect(cidrText(nextFree(space, [], 24)!)).toBe("10.0.0.0/24");
    expect(cidrText(nextFree(space, [c("10.0.0.0/24")], 24)!)).toBe("10.0.1.0/24");
    // a /25 used in the middle of the first /24 pushes a /24 to the next one, but a /26 still fits beside it
    expect(cidrText(nextFree(space, [c("10.0.0.0/25")], 24)!)).toBe("10.0.1.0/24");
    expect(cidrText(nextFree(space, [c("10.0.0.0/25")], 26)!)).toBe("10.0.0.128/26");
  });
  it("is null when nothing fits", () => {
    expect(nextFree([c("10.0.0.0/24")], [c("10.0.0.0/25")], 24)).toBeNull();
    expect(nextFree([c("10.0.0.0/24")], [], 16)).toBeNull();
    expect(nextFree([], [], 24)).toBeNull();
  });
  it("looks in every block of the space, lowest first", () => {
    expect(cidrText(nextFree([c("192.168.0.0/24"), c("10.0.0.0/24")], [c("10.0.0.0/24")], 24)!)).toBe("192.168.0.0/24");
  });
});

describe("freeAddresses", () => {
  it("counts what is left once, even when used blocks overlap each other", () => {
    const space = [c("10.0.0.0/24")];
    expect(freeAddresses(space, [])).toBe(256);
    expect(freeAddresses(space, [c("10.0.0.0/25")])).toBe(128);
    expect(freeAddresses(space, [c("10.0.0.0/25"), c("10.0.0.0/26"), c("10.0.9.0/24")])).toBe(128);
  });
});
