/* IPv4 CIDR arithmetic for the address helper (D-076). The server judges the same rules (x-tf-checks:
 * cidr, cidr_within_parent, cidr_no_overlap) with IPv6 too; here IPv6 is simply not calculated: a block that
 * is not IPv4 is left alone, never reported. Numbers are unsigned 32-bit. */

export interface Cidr {
  /** First address of the block as an unsigned integer. */
  ip: number;
  bits: number;
}

const OCTET = /^(0|[1-9]\d{0,2})$/;

export function parseCidr(text: string): Cidr | null {
  const m = text.trim().match(/^([\d.]+)\/(\d{1,2})$/);
  if (!m) return null;
  const parts = m[1].split(".");
  if (parts.length !== 4 || !parts.every((p) => OCTET.test(p) && Number(p) <= 255)) return null;
  const bits = Number(m[2]);
  if (bits > 32) return null;
  const ip = parts.reduce((a, p) => ((a << 8) | Number(p)) >>> 0, 0);
  return { ip, bits };
}

export const sizeOf = (c: Cidr): number => 2 ** (32 - c.bits);
const maskOf = (bits: number): number => (bits === 0 ? 0 : (0xffffffff << (32 - bits)) >>> 0);
export const firstOf = (c: Cidr): number => (c.ip & maskOf(c.bits)) >>> 0;
export const lastOf = (c: Cidr): number => (firstOf(c) + sizeOf(c) - 1) >>> 0;
/** True when the address is the first of its block (no host bits set). */
export const isAligned = (c: Cidr): boolean => firstOf(c) === c.ip;

export function ipText(n: number): string {
  return [n >>> 24, (n >>> 16) & 255, (n >>> 8) & 255, n & 255].join(".");
}
export const cidrText = (c: Cidr): string => `${ipText(firstOf(c))}/${c.bits}`;

export const overlaps = (a: Cidr, b: Cidr): boolean => firstOf(a) <= lastOf(b) && firstOf(b) <= lastOf(a);
export const contains = (outer: Cidr, inner: Cidr): boolean => firstOf(outer) <= firstOf(inner) && lastOf(inner) <= lastOf(outer);

/** Azure keeps the first four addresses and the last one of every subnet. */
export const AZURE_RESERVED = 5;

export interface BlockInfo {
  total: number;
  /** Addresses a resource can use in an Azure subnet. */
  usable: number;
  first: string;
  last: string;
  firstUsable: string;
  lastUsable: string;
}

export function infoOf(c: Cidr): BlockInfo {
  const total = sizeOf(c);
  const f = firstOf(c);
  const l = lastOf(c);
  return {
    total,
    usable: Math.max(0, total - AZURE_RESERVED),
    first: ipText(f),
    last: ipText(l),
    firstUsable: ipText(Math.min(l, f + 4)),
    lastUsable: ipText(Math.max(f, l - 1)),
  };
}

export type Problem = { kind: "syntax" | "host-bits" | "outside" | "overlap" | "small"; text: string };

/** A fix for a block whose address has host bits set: the block that starts where it belongs. */
export const alignedText = (c: Cidr): string => cidrText(c);

/**
 * What is wrong with a typed block, in the order the server reports it. `space` is the parent's address
 * space and `used` the blocks of the siblings (both only when they are known literals).
 */
export function problemsOf(
  text: string,
  space: Cidr[],
  used: { label: string; block: Cidr }[],
  opts: { minBits?: number } = {},
): Problem[] {
  const t = text.trim();
  if (!t) return [];
  if (t.includes(":")) return []; // IPv6 is judged by the server only
  const c = parseCidr(t);
  if (!c) return [{ kind: "syntax", text: `${t} is not a CIDR block. Write it like 10.0.1.0/24.` }];
  const out: Problem[] = [];
  if (!isAligned(c)) out.push({ kind: "host-bits", text: `${t} must start at the first address of its block. Did you mean ${alignedText(c)}?` });
  if (opts.minBits && c.bits > opts.minBits) out.push({ kind: "small", text: `A /${c.bits} is smaller than Azure allows for a subnet (/${opts.minBits} or larger).` });
  if (space.length > 0 && !space.some((s) => contains(s, c))) {
    out.push({ kind: "outside", text: `${cidrText(c)} is outside the address space (${space.map(cidrText).join(", ")}).` });
  }
  for (const u of used) if (overlaps(c, u.block)) out.push({ kind: "overlap", text: `${cidrText(c)} overlaps ${cidrText(u.block)} of ${u.label}.` });
  return out;
}

/** The first free, aligned block of `bits` inside the space that touches none of the used blocks. */
export function nextFree(space: Cidr[], used: Cidr[], bits: number): Cidr | null {
  const step = 2 ** (32 - bits);
  for (const s of [...space].sort((a, b) => firstOf(a) - firstOf(b))) {
    if (s.bits > bits) continue; // the space is smaller than the block
    for (let start = firstOf(s); start + step - 1 <= lastOf(s); start += step) {
      const cand = { ip: start >>> 0, bits };
      if (!used.some((u) => overlaps(cand, u))) return cand;
    }
  }
  return null;
}

/** The part of the space nothing uses, as a count of addresses (what is left to hand out). */
export function freeAddresses(space: Cidr[], used: Cidr[]): number {
  let free = 0;
  for (const s of space) {
    let taken = 0;
    const inside = used
      .filter((u) => overlaps(s, u))
      .map((u) => [Math.max(firstOf(s), firstOf(u)), Math.min(lastOf(s), lastOf(u))] as const)
      .sort((a, b) => a[0] - b[0]);
    let end = -1;
    for (const [a, b] of inside) {
      const from = Math.max(a, end + 1);
      if (b >= from) taken += b - from + 1;
      end = Math.max(end, b);
    }
    free += sizeOf(s) - taken;
  }
  return free;
}
