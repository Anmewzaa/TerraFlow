/* The left palette (M4b): search, category and provider filters over modules and raw resources.
 * Pure functions, so the rules are tested without a screen. */

import type { ModuleSummary } from "../types/catalog";
import { fitToProject, providerOfType, type Category, type ProjectProvider, type Versioned } from "./provider";

export const OTHER = "other";

/** Every word of the query must appear in the text (any order, any case). */
export function matches(query: string, ...fields: (string | undefined)[]): boolean {
  const words = query.trim().toLowerCase().split(/\s+/).filter(Boolean);
  if (words.length === 0) return true;
  const hay = fields.join(" ").toLowerCase();
  return words.every((w) => hay.includes(w));
}

export interface PaletteFilter {
  query: string;
  /** "" means every category. */
  category: string;
  providers: ProjectProvider[] | undefined;
}

/** A module's category: its own, else "other" (the server already maps older schemas by their domain). */
export const categoryOfModule = (m: ModuleSummary): string => m.category || OTHER;

export function paletteModules(
  modules: ModuleSummary[],
  f: PaletteFilter,
): { shown: ModuleSummary[]; hiddenByProvider: number } {
  const chosen = new Set((f.providers ?? []).map((p) => p.id));
  let hiddenByProvider = 0;
  const shown: ModuleSummary[] = [];
  for (const m of modules) {
    // A module with no provider fits every project; one written for a provider needs the project to use it.
    if (chosen.size > 0 && m.provider && !chosen.has(m.provider)) {
      hiddenByProvider++;
      continue;
    }
    if (f.category && categoryOfModule(m) !== f.category) continue;
    if (!matches(f.query, m.title, m.name, m.description)) continue;
    shown.push(m);
  }
  return { shown, hiddenByProvider };
}

export function paletteRaw<T extends Versioned & { resource_type: string; title: string; description?: string; category?: string }>(
  templates: T[],
  f: PaletteFilter,
): { shown: T[]; hidden: number } {
  const fit = fitToProject(templates, f.providers);
  const shown = fit.shown.filter(
    (t) => (!f.category || (t.category || OTHER) === f.category) && matches(f.query, t.title, t.resource_type, t.description),
  );
  return { shown, hidden: fit.hidden.length };
}

/** Items grouped by category in the server's display order; an unknown category goes to "other". */
export function groupByCategory<T>(items: T[], categoryOf: (t: T) => string, order: Category[]): { category: Category; items: T[] }[] {
  const known = new Map(order.map((c) => [c.id, c]));
  const buckets = new Map<string, T[]>();
  for (const it of items) {
    const id = known.has(categoryOf(it)) ? categoryOf(it) : OTHER;
    buckets.set(id, [...(buckets.get(id) ?? []), it]);
  }
  const fallback: Category = known.get(OTHER) ?? { id: OTHER, label: "Other" };
  return [...order.map((c) => c.id), OTHER]
    .filter((id, i, all) => all.indexOf(id) === i && buckets.has(id))
    .map((id) => ({ category: known.get(id) ?? fallback, items: buckets.get(id)! }));
}

/** The categories that have something to show, so a chip never leads to an empty list. */
export function usedCategories(modulesIn: ModuleSummary[], rawIn: { category?: string }[], order: Category[]): Category[] {
  const used = new Set([...modulesIn.map(categoryOfModule), ...rawIn.map((r) => r.category || OTHER)]);
  return order.filter((c) => used.has(c.id));
}

export { providerOfType };
