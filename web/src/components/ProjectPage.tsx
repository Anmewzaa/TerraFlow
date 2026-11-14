import { useEffect, useState, type ReactNode } from "react";
import type { User } from "../types/auth";
import type { ModuleSummary } from "../types/catalog";
import { ProjectSettingsView } from "./ProjectSettingsView";
import { TeamView } from "./TeamView";
import { canApproveRole, useTeam } from "../hooks/useGitHub";
import { catalogPaths } from "../lib/catalogApi";
import { ModulesPanel } from "./admin/ModulesPanel";
import { RawResourcesPanel } from "./admin/RawResourcesPanel";
import { BaseBranchPanel } from "./BaseBranchPanel";
import { buildProjectHash, parseProjectHash, type ProjectTab } from "../lib/projectHash";

const SECTIONS: { id: string; label: string; group: string; tab: ProjectTab }[] = [
  { id: "team", label: "Team and repository", group: "People", tab: "project" },
  { id: "branch", label: "Branch", group: "Settings", tab: "settings" },
  { id: "providers", label: "Cloud providers", group: "Settings", tab: "settings" },
  { id: "environments", label: "Environments", group: "Settings", tab: "settings" },
  { id: "checks", label: "Checks", group: "Settings", tab: "settings" },
  { id: "folders", label: "Where an environment lives", group: "Settings", tab: "settings" },
  { id: "layout", label: "Where new resources go", group: "Settings", tab: "settings" },
  { id: "variables", label: "Variables", group: "Settings", tab: "settings" },
  { id: "starters", label: "Starter files", group: "Settings", tab: "settings" },
  { id: "catalog-modules", label: "Project modules", group: "Catalog", tab: "settings" },
  { id: "catalog-resources", label: "Project raw resources", group: "Catalog", tab: "settings" },
];

/** A project member who is refused sees the server's message in the panel; there is nowhere else to send them. */
const denied = () => {};

function jump(id: string) {
  document.getElementById(id)?.scrollIntoView({ behavior: "smooth", block: "start" });
}

function Nav({ tab }: { tab: ProjectTab }): ReactNode {
  const shown = SECTIONS.filter((s) => s.tab === tab);
  const groups = [...new Set(shown.map((s) => s.group))];
  return (
    <nav aria-label="On this page" className="md:sticky md:top-0 md:self-start">
      {groups.map((g) => (
        <div key={g} className="mb-4">
          <p className="m-0 mb-1 text-[11px] font-semibold uppercase tracking-wider text-[var(--tf-color-text-muted)]">{g}</p>
          <ul className="m-0 flex list-none gap-1 overflow-x-auto p-0 md:block">
            {shown.filter((s) => s.group === g).map((s) => (
              <li key={s.id}>
                <button
                  onClick={() => jump(s.id)}
                  className="w-full whitespace-nowrap rounded-md px-2 py-1.5 text-left text-sm text-[var(--tf-color-text-secondary)] hover:bg-[var(--tf-glass-hover)] hover:text-[var(--tf-color-text)]"
                >
                  {s.label}
                </button>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </nav>
  );
}

interface Props {
  projectId: string;
  user: User | null;
  modules: ModuleSummary[];
  isAdmin: boolean;
  onDeleted: () => void;
  onSaved: () => void;
  /** Called when the project's own catalog changed, so the palette reloads. */
  onCatalogChanged: () => void;
}

/**
 * One page for looking after a project: its team first (who may do what), then its settings
 * (providers, layout, variables, starter files) and its own catalog (D-060) below. Both were separate screens; they are
 * the same job for the same people, and a contents list on the left keeps them one click apart.
 */
export function ProjectPage({ projectId, user, modules, isAdmin, onDeleted, onSaved, onCatalogChanged }: Props) {
  const { team } = useTeam(projectId);
  // The owner and co-owners manage the project's own catalog (D-060); an admin may too.
  const canManage = isAdmin || canApproveRole(team?.my_role);
  const paths = catalogPaths(projectId);
  // The tab lives in the URL hash (#/p/<id>/settings), so a refresh or a shared link lands on the same one.
  const [tab, setTabState] = useState<ProjectTab>(() => parseProjectHash(window.location.hash).tab);
  const setTab = (next: ProjectTab) => {
    setTabState(next);
    window.location.hash = buildProjectHash(projectId, "project", next);
  };
  useEffect(() => {
    const onHash = () => setTabState(parseProjectHash(window.location.hash).tab);
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  }, []);
  return (
    <div className="flex-1 overflow-y-auto">
      <div role="tablist" aria-label="Project pages" className="mx-auto flex max-w-6xl gap-1 px-6 pt-6 md:px-8">
        {(["project", "settings"] as const).map((id) => (
          <button
            key={id}
            role="tab"
            aria-selected={tab === id}
            onClick={() => setTab(id)}
            className={`rounded-md px-4 py-2 text-sm font-medium ${tab === id ? "bg-[var(--tf-glass-hover)] text-[var(--tf-color-text)]" : "text-[var(--tf-color-text-secondary)] hover:bg-[var(--tf-glass-hover)]"}`}
          >
            {id === "project" ? "Project" : "Settings"}
          </button>
        ))}
      </div>
      <div className="mx-auto grid max-w-6xl grid-cols-[minmax(0,1fr)] gap-6 p-6 md:grid-cols-[13rem_minmax(0,1fr)] md:gap-10 md:p-8">
        <Nav tab={tab} />
        <div className="min-w-0 space-y-12">
          {tab === "project" && (
            <div id="team" className="scroll-mt-4">
              <TeamView projectId={projectId} user={user} onDeleted={onDeleted} embedded />
            </div>
          )}
          {tab === "settings" && (
            <div className="scroll-mt-4">
              <BaseBranchPanel projectId={projectId} canManage={canManage} />
            </div>
          )}
          {tab === "settings" && (
          <>
          <div id="settings" className="scroll-mt-4 border-t border-[var(--tf-color-border)] pt-10">
            <ProjectSettingsView projectId={projectId} modules={modules} isAdmin={isAdmin} onSaved={onSaved} embedded />
          </div>
          <div id="catalog-modules" className="scroll-mt-4 border-t border-[var(--tf-color-border)] pt-10">
            {canManage ? (
              <ModulesPanel paths={paths} onDenied={denied} onChanged={onCatalogChanged} />
            ) : (
              <div>
                <h2 className="m-0 text-xl font-semibold tracking-tight">Project catalog</h2>
                <p className="m-0 mt-1 max-w-2xl text-sm text-[var(--tf-color-text-secondary)]">
                  This project can have its own modules and raw resources next to the global catalog. The owner and co-owners add them; you
                  find them in the palette, marked "project".
                </p>
              </div>
            )}
          </div>
          {canManage && (
            <div id="catalog-resources" className="scroll-mt-4 border-t border-[var(--tf-color-border)] pt-10">
              <RawResourcesPanel paths={paths} onDenied={denied} onChanged={onCatalogChanged} />
            </div>
          )}
          </>
          )}
        </div>
      </div>
    </div>
  );
}
