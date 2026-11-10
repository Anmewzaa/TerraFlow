import type { ReactNode } from "react";
import type { User } from "../types/auth";
import type { ModuleSummary } from "../types/catalog";
import { ProjectSettingsView } from "./ProjectSettingsView";
import { TeamView } from "./TeamView";

const SECTIONS: { id: string; label: string; group: string }[] = [
  { id: "team", label: "Team", group: "People" },
  { id: "providers", label: "Cloud providers", group: "Settings" },
  { id: "layout", label: "Where new resources go", group: "Settings" },
  { id: "variables", label: "Variables", group: "Settings" },
  { id: "starters", label: "Starter files", group: "Settings" },
];

function jump(id: string) {
  document.getElementById(id)?.scrollIntoView({ behavior: "smooth", block: "start" });
}

function Nav(): ReactNode {
  const groups = [...new Set(SECTIONS.map((s) => s.group))];
  return (
    <nav aria-label="On this page" className="md:sticky md:top-0 md:self-start">
      {groups.map((g) => (
        <div key={g} className="mb-4">
          <p className="m-0 mb-1 text-[11px] font-semibold uppercase tracking-wider text-[var(--tf-color-text-muted)]">{g}</p>
          <ul className="m-0 flex list-none gap-1 overflow-x-auto p-0 md:block">
            {SECTIONS.filter((s) => s.group === g).map((s) => (
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
}

/**
 * One page for looking after a project: its team first (who may do what), then its settings
 * (providers, layout, variables, starter files) below. Both were separate screens; they are
 * the same job for the same people, and a contents list on the left keeps them one click apart.
 */
export function ProjectPage({ projectId, user, modules, isAdmin, onDeleted, onSaved }: Props) {
  return (
    <div className="flex-1 overflow-y-auto">
      <div className="mx-auto grid max-w-6xl gap-6 p-6 md:grid-cols-[13rem_minmax(0,1fr)] md:gap-10 md:p-8">
        <Nav />
        <div className="min-w-0 space-y-12">
          <div id="team" className="scroll-mt-4">
            <TeamView projectId={projectId} user={user} onDeleted={onDeleted} embedded />
          </div>
          <div id="settings" className="scroll-mt-4 border-t border-[var(--tf-color-border)] pt-10">
            <ProjectSettingsView projectId={projectId} modules={modules} isAdmin={isAdmin} onSaved={onSaved} embedded />
          </div>
        </div>
      </div>
    </div>
  );
}
