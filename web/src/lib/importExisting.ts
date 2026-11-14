import type { CanvasNode } from "../types/changeset";

/** What GET /projects/{id}/import returns for one module call. */
export interface ImportedNode {
  id: string;
  module_name: string;
  instance_name: string;
  target_folder: string;
  source?: string;
  filename?: string;
  /** The catalog version that describes this block, chosen by the server (D-065). */
  module_version?: string;
  values: Record<string, unknown>;
  existing: { folder: string; filename: string; label: string };
}

/** The version a module source pins, e.g. "v1.1.0" from "...//rg?ref=v1.1.0". */
export function versionFromSource(source?: string): string {
  const m = source?.match(/\?ref=([^&]+)/);
  return m ? m[1] : "";
}

/**
 * The catalog version whose schema describes a module pinned at `wanted` in the
 * repository (e.g. v1.2.0): the same version, else the same major (v1), else
 * the latest. The repository's own source line is kept when it is written back.
 */
export function resolveCatalogVersion(
  wanted: string,
  catalog: { versions: string[]; latest_version: string } | undefined,
): string {
  if (!catalog) return wanted;
  if (catalog.versions.includes(wanted)) return wanted;
  const major = wanted.match(/^v?(\d+)/)?.[1];
  if (major) {
    const same = catalog.versions.filter((v) => v === `v${major}` || v.startsWith(`v${major}.`));
    if (same.length) return same[same.length - 1];
  }
  return catalog.latest_version || wanted;
}

/** Place the n-th imported node on a simple grid so they do not stack. */
export function toCanvasNode(
  imp: ImportedNode,
  index: number,
  catalog?: { versions: string[]; latest_version: string },
): CanvasNode {
  return {
    id: imp.id,
    module_name: imp.module_name,
    // The server decides which schema describes the block; the local rule is only for an older server.
    module_version: imp.module_version || resolveCatalogVersion(versionFromSource(imp.source), catalog),
    instance_name: imp.instance_name,
    target_folder: imp.target_folder,
    values: imp.values ?? {},
    position_x: 40 + (index % 4) * 280,
    position_y: 40 + Math.floor(index / 4) * 180,
    filename: imp.filename,
    existing: imp.existing,
    source: imp.source,
  };
}
