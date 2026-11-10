/* Cloud providers (D-058): which raw resources fit a project. Mirrors server/internal/hcl/provider.go;
 * a help for choosing, not a guarantee: TerraFlow does not read a provider's own schema. */

export interface RegisteredProvider {
  id: string;
  name: string;
  source: string;
  logo: string;
  versions: string[];
  block: string;
}

export interface Category {
  id: string;
  label: string;
}

export interface ProjectProvider {
  id: string;
  version?: string;
}

export interface Versioned {
  provider?: string;
  min_version?: string;
  max_version?: string;
}

/** The provider a resource type belongs to: azurerm_resource_group -> azurerm. */
export const providerOfType = (type: string): string => {
  const i = type.indexOf("_");
  return i > 0 ? type.slice(0, i) : "";
};

function numbers(v: string): [number, number] | null {
  const m = v.match(/\d+/g);
  return m ? [Number(m[0]), Number(m[1] ?? 0)] : null;
}
const before = (a: [number, number], b: [number, number]) => a[0] < b[0] || (a[0] === b[0] && a[1] < b[1]);

/** True when a resource available from min up to max can be used with the project's version constraint. */
export function compatible(projectConstraint: string | undefined, min?: string, max?: string): boolean {
  const p = projectConstraint ? numbers(projectConstraint) : null;
  if (!p) return true; // no version, or one we cannot read: hide nothing
  const lo = min ? numbers(min) : null;
  const hi = max ? numbers(max) : null;
  if (lo && before(p, lo)) return false;
  if (hi && before(hi, p)) return false;
  return true;
}

/**
 * Splits raw resources into those that fit the project's providers and those that do not. A project
 * with no provider chosen sees everything (nothing was said, so nothing is hidden).
 */
export function fitToProject<T extends Versioned & { resource_type: string }>(
  items: T[],
  providers: ProjectProvider[] | undefined,
): { shown: T[]; hidden: T[] } {
  if (!providers || providers.length === 0) return { shown: items, hidden: [] };
  const shown: T[] = [];
  const hidden: T[] = [];
  for (const t of items) {
    const prov = t.provider || providerOfType(t.resource_type);
    const chosen = providers.find((p) => p.id === prov);
    (chosen && compatible(chosen.version, t.min_version, t.max_version) ? shown : hidden).push(t);
  }
  return { shown, hidden };
}
