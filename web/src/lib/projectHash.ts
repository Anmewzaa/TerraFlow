/**
 * The URL hash of a project, so a refresh, the back button and a shared link land on the same screen:
 * `#/p/<id>` is the canvas; `#/p/<id>/changesets` (changes and their review), `/hcl` and `/compare` are the other
 * screens of the rail; `#/p/<id>/team` and `#/p/<id>/settings` are the two tabs of the project page.
 */
export type ProjectTab = "project" | "settings";

/** A rail view other than the canvas; "project" is the project page (see `tab`). */
export type ProjectView = "canvas" | "changesets" | "hcl" | "compare" | "project";

export interface ProjectHash {
  projectId: string | null;
  view: ProjectView;
  tab: ProjectTab;
}

const PLAIN_VIEWS = ["changesets", "hcl", "compare"] as const;

export function parseProjectHash(hash: string): ProjectHash {
  const m = hash.match(/^#\/p\/([^/]+)(?:\/([a-z]+))?$/);
  const none: ProjectHash = { projectId: null, view: "canvas", tab: "project" };
  if (!m) return none;
  let id: string;
  try {
    id = decodeURIComponent(m[1]);
  } catch {
    return none;
  }
  const seg = m[2];
  if (!seg) return { projectId: id, view: "canvas", tab: "project" };
  if (seg === "team") return { projectId: id, view: "project", tab: "project" };
  if (seg === "settings") return { projectId: id, view: "project", tab: "settings" };
  // Review was merged into Changesets: an old link to it lands there.
  if (seg === "review") return { projectId: id, view: "changesets", tab: "project" };
  if ((PLAIN_VIEWS as readonly string[]).includes(seg)) return { projectId: id, view: seg as ProjectView, tab: "project" };
  return none;
}

export function buildProjectHash(projectId: string | null, view: ProjectView = "canvas", tab: ProjectTab = "project"): string {
  if (!projectId) return "";
  const base = `/p/${encodeURIComponent(projectId)}`;
  if (view === "canvas") return base;
  if (view === "project") return `${base}/${tab === "settings" ? "settings" : "team"}`;
  return `${base}/${view}`;
}
