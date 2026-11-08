import { describe, expect, it } from "vitest";
import { diffLines } from "./diff";
import { blockAt, blocksOf, condense } from "./hclBlocks";

const before = `module "rg" {
  source = "x"
  name   = "rg-a"
}

module "vnet" {
  source = "y"
  name   = "vnet-a"
}
`;

const after = `module "rg" {
  source = "x"
  name   = "rg-a"
}

module "vnet" {
  source = "y"
  name   = "vnet-b"
}

# keep it small
resource "azurerm_resource_group" "extra" {
  name = "rg-extra"
}
`;

describe("blocksOf", () => {
  const d = diffLines(before, after);
  const blocks = blocksOf(d.text, d.kinds);

  it("names components and says what changed", () => {
    expect(blocks.map((b) => [b.kind, b.name, b.change])).toEqual([
      ["module", "rg", "unchanged"],
      ["module", "vnet", "modified"],
      ["resource", "azurerm_resource_group.extra", "added"],
    ]);
  });

  it("a comment directly above a block belongs to it", () => {
    const extra = blocks[2];
    expect(d.text.split("\n")[extra.start]).toBe("# keep it small");
  });
});

describe("condense", () => {
  it("drops unchanged blocks and keeps changed ones with their kinds", () => {
    const d = diffLines(before, after);
    const c = condense(d.text, d.kinds, d.previous, blocksOf(d.text, d.kinds));
    expect(c.text).toContain("# ... 1 unchanged: rg");
    expect(c.text).toContain('module "vnet"');
    expect(c.text).toContain('resource "azurerm_resource_group" "extra"');
    expect(c.text).not.toContain('module "rg"');
    expect(c.kinds.length).toBe(c.text.split("\n").length);
    expect(c.kinds).toContain("mod");
    expect(c.kinds).toContain("add");
  });

  it("a brand-new file keeps everything", () => {
    const d = diffLines("", after);
    const blocks = blocksOf(d.text, d.kinds);
    expect(blocks.every((b) => b.change === "added")).toBe(true);
    expect(condense(d.text, d.kinds, d.previous, blocks).text).toContain('module "rg"');
  });
});

describe("block details", () => {
  it("counts the lines that changed inside each block and finds the block of a line", () => {
    const d = diffLines(before, after);
    const blocks = blocksOf(d.text, d.kinds);
    const vnet = blocks.find((b) => b.name === "vnet")!;
    expect(vnet.stats).toEqual({ add: 0, mod: 1, del: 0 });
    const extra = blocks.find((b) => b.name === "azurerm_resource_group.extra")!;
    expect(extra.stats.add).toBe(3); // header, one argument, closing brace
    expect(blockAt(blocks, vnet.start + 1)).toBe(blocks.indexOf(vnet));
    expect(blockAt(blocks, d.text.split("\n").length + 5)).toBe(-1);
  });
});
