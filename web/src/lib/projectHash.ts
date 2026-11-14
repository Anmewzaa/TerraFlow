/** The URL hash of a project: `#/p/<id>` or `#/p/<id>/<tab>`, where tab is a Settings screen tab (8.8). */
export type ProjectTab = "project" | "settings";

export interface ProjectHash {
  projectId: string | null;
  tab: ProjectTab;
}

export function parseProjectHash(hash: string): ProjectHash {
  const m = hash.match(/^#\/p\/([^/]+)(?:\/(project|settings))?$/);
  if (!m) return { projectId: null, tab: "project" };
  let id: string;
  try {
    id = decodeURIComponent(m[1]);
  } catch {
    return { projectId: null, tab: "project" };
  }
  return { projectId: id, tab: (m[2] as ProjectTab | undefined) ?? "project" };
}

/** The first tab is the default, so it stays out of the URL. */
export function buildProjectHash(projectId: string | null, tab: ProjectTab = "project"): string {
  if (!projectId) return "";
  const base = `/p/${encodeURIComponent(projectId)}`;
  return tab === "project" ? base : `${base}/${tab}`;
}
