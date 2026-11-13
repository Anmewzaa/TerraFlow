/**
 * Where a catalog lives (D-060). Without a project it is the global catalog, managed by an
 * admin; with one it is what that project sees (global plus its own) and the project's own
 * part is managed by its owner and co-owners. Paths are relative to /api.
 */
export interface CatalogPaths {
  scope: "global" | "project";
  /** List of module summaries; `${read}/{name}/{version}` is one module with its schema. */
  read: string;
  /** `${manage}/modules`, `/validate` and `/draft`. */
  manage: string;
  /** The raw resource templates to read, and where they are created, changed and deleted. */
  resources: string;
  resourcesManage: string;
}

export function catalogPaths(projectId?: string | null): CatalogPaths {
  if (!projectId) {
    return { scope: "global", read: "/catalog", manage: "/admin/catalog", resources: "/catalog/resources", resourcesManage: "/admin/resources" };
  }
  const base = `/projects/${encodeURIComponent(projectId)}/catalog`;
  return { scope: "project", read: base, manage: base, resources: `${base}/resources`, resourcesManage: `${base}/resources` };
}
