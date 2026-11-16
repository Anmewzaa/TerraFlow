import { useMemo } from "react";
import { Wand2 } from "lucide-react";
import { ListEditor } from "./ListEditor";
import { useFormNode, useGraphContext } from "../../../contexts/GraphContext";
import { cidrText, freeAddresses, infoOf, nextFree, parseCidr, problemsOf, type Cidr } from "../../../lib/cidr";

interface Props {
  value: string[];
  onChange: (value: unknown) => void;
  /** x-tf-ui.cidr: parent_field names the input of the parent that holds the address space; suggest lists prefix lengths. */
  config?: { parent_field?: string; suggest?: number[] };
}

const REFERENCE = /^\{[a-z_][a-z0-9_]*\}$/;

/** Literal blocks of a value that is a list of text; null when any part cannot be known (a reference or a variable). */
function literalBlocks(v: unknown): Cidr[] | null {
  if (v == null) return [];
  if (!Array.isArray(v)) return null;
  const out: Cidr[] = [];
  for (const it of v) {
    if (typeof it !== "string") return null;
    if (!it.trim()) continue;
    if (REFERENCE.test(it.trim())) return null;
    const c = parseCidr(it);
    if (!c) return null;
    out.push(c);
  }
  return out;
}

const STARTERS = ["10.0.0.0/16", "172.16.0.0/16", "192.168.0.0/24"];

/**
 * A list of CIDR blocks with a calculator: what each block holds, what is wrong with it (the same rules the
 * server enforces), and, inside a network, which blocks are free so the next one is a click away.
 */
export function CidrListEditor({ value, onChange, config }: Props) {
  const nodeId = useFormNode();
  const { draftNodes } = useGraphContext();
  const entries = (value ?? []).map(String);

  const ctx = useMemo(() => {
    const me = draftNodes.find((n) => n.id === nodeId);
    const field = config?.parent_field;
    if (!me || !field || !me.parent_id) return null;
    const parent = draftNodes.find((n) => n.id === me.parent_id);
    if (!parent) return null;
    const space = literalBlocks(parent.values?.[field]);
    const siblings = draftNodes.filter((n) => n.id !== me.id && n.parent_id === me.parent_id && n.module_name === me.module_name);
    const used: { label: string; block: Cidr }[] = [];
    let usedKnown = true;
    for (const s of siblings) {
      const label = typeof s.values?.name === "string" && s.values.name ? s.values.name : s.instance_name;
      const blocks = literalBlocks(s.values?.address_prefixes);
      if (blocks === null) usedKnown = false;
      else for (const b of blocks) used.push({ label, block: b });
    }
    const pname = typeof parent.values?.name === "string" && parent.values.name ? parent.values.name : parent.instance_name;
    return { space: space ?? [], spaceKnown: space !== null && space.length > 0, used, usedKnown, parentName: pname };
  }, [draftNodes, nodeId, config?.parent_field]);

  const space = ctx?.spaceKnown ? ctx.space : [];
  const mine = entries.map((e) => parseCidr(e)).filter((c): c is Cidr => !!c);
  // The other entries of this same field count as used too, so two blocks typed here do not collide.
  const usedFor = (index: number) => [
    ...(ctx?.used ?? []),
    ...entries.flatMap((e, i) => {
      const c = i === index ? null : parseCidr(e);
      return c ? [{ label: "another block of this subnet", block: c }] : [];
    }),
  ];

  const add = (c: Cidr) => onChange([...entries.filter((e) => e.trim()), cidrText(c)]);
  const sizes = config?.suggest ?? [];
  const taken = [...(ctx?.used.map((u) => u.block) ?? []), ...mine];
  const suggestions = ctx?.spaceKnown && ctx.usedKnown ? sizes.map((bits) => ({ bits, block: nextFree(ctx.space, taken, bits) })) : [];
  const free = ctx?.spaceKnown && ctx.usedKnown ? freeAddresses(ctx.space, taken) : null;

  return (
    <div style={{ display: "grid", gap: 8 }}>
      <ListEditor value={entries} onChange={onChange} />

      {entries.filter((e) => e.trim()).length > 0 && (
        <ul style={{ margin: 0, padding: 0, listStyle: "none", display: "grid", gap: 4 }} aria-label="Address details">
          {entries.map((e, i) => {
            if (!e.trim()) return null;
            const problems = problemsOf(e, space, usedFor(i), { minBits: ctx ? 29 : undefined });
            const c = parseCidr(e);
            return (
              <li key={i} style={{ fontSize: "var(--tf-text-caption)", lineHeight: 1.45 }}>
                {problems.length > 0 ? (
                  problems.map((p) => (
                    <div key={p.kind} role="alert" style={{ color: "var(--tf-color-error)" }}>
                      {p.text}
                    </div>
                  ))
                ) : c ? (
                  <div style={{ color: "var(--tf-color-text-muted)" }}>
                    <code style={{ color: "var(--tf-color-text)" }}>{cidrText(c)}</code> · {infoOf(c).total.toLocaleString()} addresses
                    {ctx ? ` · ${infoOf(c).usable.toLocaleString()} usable` : ""} · {infoOf(c).first} to {infoOf(c).last}
                  </div>
                ) : null}
              </li>
            );
          })}
        </ul>
      )}

      {ctx && !ctx.spaceKnown && (
        <p style={{ margin: 0, fontSize: "var(--tf-text-caption)", color: "var(--tf-color-text-muted)" }}>
          The address space of {ctx.parentName} is not typed here (a reference or a variable), so the range is not checked on the canvas.
        </p>
      )}

      {ctx?.spaceKnown && (
        <div className="tf-cidr-helper" style={{ border: "1px dashed var(--tf-color-border-strong)", borderRadius: "var(--tf-radius-control)", padding: "8px 10px", display: "grid", gap: 6 }}>
          <div style={{ fontSize: "var(--tf-text-caption)", color: "var(--tf-color-text-muted)" }}>
            Address space of <strong style={{ color: "var(--tf-color-text)" }}>{ctx.parentName}</strong>: {ctx.space.map(cidrText).join(", ")}
            {free !== null && <> · {free.toLocaleString()} addresses free</>}
          </div>
          {ctx.used.length > 0 && (
            <div style={{ fontSize: "var(--tf-text-caption)", color: "var(--tf-color-text-muted)" }}>
              In use: {ctx.used.map((u) => `${u.label} ${cidrText(u.block)}`).join(" · ")}
            </div>
          )}
          {suggestions.length > 0 && (
            <div style={{ display: "flex", flexWrap: "wrap", gap: 6, alignItems: "center" }}>
              <Wand2 size={13} aria-hidden style={{ color: "var(--tf-color-text-muted)" }} />
              {suggestions.map(({ bits, block }) =>
                block ? (
                  <button key={bits} type="button" className="tf-btn tf-btn--ghost" style={{ height: 26, padding: "0 8px", fontSize: 12 }} onClick={() => add(block)} title={`Add the next free /${bits}`}>
                    /{bits} · {cidrText(block)}
                  </button>
                ) : (
                  <span key={bits} style={{ fontSize: 12, color: "var(--tf-color-text-muted)" }}>
                    no free /{bits}
                  </span>
                ),
              )}
            </div>
          )}
        </div>
      )}

      {!ctx && entries.every((e) => !e.trim()) && (
        <div style={{ display: "flex", flexWrap: "wrap", gap: 6, alignItems: "center", fontSize: 12, color: "var(--tf-color-text-muted)" }}>
          Start from
          {STARTERS.map((s) => (
            <button key={s} type="button" className="tf-btn tf-btn--ghost" style={{ height: 26, padding: "0 8px", fontSize: 12 }} onClick={() => onChange([s])}>
              {s}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
