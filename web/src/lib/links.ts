import type { LinkOffer, ModuleSummary } from "../types/catalog";
import type { ProjectGraph } from "../types/graph";
import type { CanvasNode } from "../types/changeset";
import { valueText } from "./valueText";

/** A resource on the canvas or in the repository that a field asking for a link may point at. */
export interface LinkTarget {
  /** The block label the reference uses: module.<label>.<output>. */
  label: string;
  name: string;
  folder: string;
  draft: boolean;
  offer: LinkOffer;
  /** The expression to write when it is not module.<label>.<output> (an offer through the parent). */
  expr?: string;
}

/** Every resource that offers `kind`, in the repository and in the draft. Nothing is guessed from names (rule 4). */
export function linkTargets(kind: string, modules: ModuleSummary[], graph: ProjectGraph | null, drafts: CanvasNode[] | undefined): LinkTarget[] {
  const offerOf = new Map<string, LinkOffer>();
  for (const m of modules) {
    const o = m.offers?.find((x) => x.kind === kind);
    if (o) offerOf.set(m.name, o);
  }
  const out: LinkTarget[] = [];
  for (const f of graph?.folders ?? []) {
    for (const n of f.nodes) {
      const offer = n.kind === "module" ? offerOf.get(n.type) : undefined;
      if (offer && n.label) out.push({ label: n.label, name: n.name ?? n.label, folder: f.name, draft: false, offer });
    }
  }
  for (const d of drafts ?? []) {
    const offer = offerOf.get(d.module_name);
    if (offer && d.instance_name && !d.existing) {
      const name = valueText(d.values?.name) || d.instance_name;
      if (offer.via_parent) {
        // A subnet is reached through the virtual network it is placed in.
        const parent = (drafts ?? []).find((p) => p.id === d.parent_id);
        if (parent?.instance_name) {
          out.push({ label: d.instance_name, name, folder: d.target_folder, draft: true, offer, expr: `module.${parent.instance_name}.${offer.output.replace("{name}", name)}` });
        }
        continue;
      }
      out.push({ label: d.instance_name, name, folder: d.target_folder, draft: true, offer });
    }
  }
  return out;
}

export const linkExpr = (t: LinkTarget): string => t.expr ?? `module.${t.label}.${t.offer.output}`;

/** The reference a link field holds now ("" when it holds none), whether it is one expression or a list of them. */
export function currentLink(value: unknown): string {
  const first = Array.isArray(value) ? value[0] : value;
  if (first && typeof first === "object" && "$expr" in (first as object)) return String((first as { $expr: unknown }).$expr);
  return "";
}

/** What picking a target writes into the field and into the input it fills. */
export function linkValue(t: LinkTarget, list: boolean | undefined, fills: string | undefined): { value: unknown; siblings?: Record<string, unknown> } {
  const ref = { $expr: linkExpr(t) };
  return {
    value: list ? [ref] : ref,
    siblings: fills && t.offer.subresource ? { [fills]: [t.offer.subresource] } : undefined,
  };
}
