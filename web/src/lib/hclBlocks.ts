import type { LineKind } from "./diff";

/** One top-level block of a generated .tf file, with what changed in it. */
export interface HclBlock {
  /** module | resource | data | variable | output | locals | provider | terraform */
  kind: string;
  /** What it is called in HCL: vnet, azurerm_resource_group.rg, data.azurerm_x.y */
  name: string;
  /** First line (comments directly above the block belong to it) and last line, 0-based. */
  start: number;
  end: number;
  change: "added" | "modified" | "removed" | "unchanged";
  /** Lines added / modified / removed inside the block. */
  stats: { add: number; mod: number; del: number };
}

const HEADER = /^(module|resource|data|variable|output|locals|provider|terraform)\b([^{]*)\{\s*$/;

function quoted(rest: string): string[] {
  return [...rest.matchAll(/"([^"]*)"/g)].map((m) => m[1]);
}

/** Find the top-level blocks of `text` (a block starts and ends at column 0). */
export function blocksOf(text: string, kinds: LineKind[] = []): HclBlock[] {
  const lines = text.split("\n");
  const out: HclBlock[] = [];
  let i = 0;
  while (i < lines.length) {
    const m = lines[i].match(HEADER);
    if (!m) {
      i++;
      continue;
    }
    let end = i;
    while (end + 1 < lines.length && lines[end].trimEnd() !== "}") end++;
    let start = i;
    const floor = out.length ? out[out.length - 1].end + 1 : 0;
    while (start - 1 >= floor && lines[start - 1].startsWith("#")) start--;

    const labels = quoted(m[2]);
    const name = m[1] === "data" ? ["data", ...labels].join(".") : labels.join(".") || m[1];

    const inBlock = kinds.slice(i, end + 1);
    const touched = inBlock.filter((k) => k && k !== "same");
    let change: HclBlock["change"] = "unchanged";
    if (touched.length > 0) {
      if (kinds[i] === "add") change = "added";
      else if (kinds[i] === "del") change = "removed";
      else change = "modified";
    }
    const stats = {
      add: inBlock.filter((k) => k === "add").length,
      mod: inBlock.filter((k) => k === "mod").length,
      del: inBlock.filter((k) => k === "del").length,
    };
    out.push({ kind: m[1], name, start, end, change, stats });
    i = end + 1;
  }
  return out;
}

export interface Condensed {
  text: string;
  kinds: LineKind[];
  previous: (string | undefined)[];
}

/**
 * Keep only the blocks that were added, changed or removed. A run of unchanged
 * blocks becomes one `# ... N unchanged: a, b` line so the reader still sees
 * that something is there.
 */
export function condense(
  text: string,
  kinds: LineKind[],
  previous: (string | undefined)[],
  blocks: HclBlock[],
): Condensed {
  const lines = text.split("\n");
  const outLines: string[] = [];
  const outKinds: LineKind[] = [];
  const outPrev: (string | undefined)[] = [];
  const push = (l: string, k: LineKind, p?: string) => {
    outLines.push(l);
    outKinds.push(k);
    outPrev.push(p);
  };

  let run: HclBlock[] = [];
  const flush = () => {
    if (run.length === 0) return;
    const names = run.map((b) => b.name);
    const shown = names.length > 3 ? `${names.slice(0, 3).join(", ")}, +${names.length - 3} more` : names.join(", ");
    push(`# ... ${run.length} unchanged: ${shown}`, "same");
    push("", "same");
    run = [];
  };

  for (const b of blocks) {
    if (b.change === "unchanged") {
      run.push(b);
      continue;
    }
    flush();
    for (let i = b.start; i <= b.end; i++) push(lines[i], kinds[i] ?? "same", previous[i]);
    push("", "same");
  }
  flush();
  if (outLines[outLines.length - 1] === "") {
    outLines.pop();
    outKinds.pop();
    outPrev.pop();
  }
  return { text: outLines.join("\n"), kinds: outKinds, previous: outPrev };
}

/** The block that contains line `line` (0-based) of the displayed text, if any. */
export function blockAt(blocks: HclBlock[], line: number): number {
  return blocks.findIndex((b) => line >= b.start && line <= b.end);
}
