import { useCallback, useState, type ReactNode } from "react";
import { Boxes, Cloud, FolderGit2, Shapes, Users } from "lucide-react";
import { DocsPage } from "./admin/DocsPage";
import type { HelpTopic } from "./admin/helpContent";
import { ModulesPanel } from "./admin/ModulesPanel";
import { PeoplePanel } from "./admin/PeoplePanel";
import { ProjectsPanel } from "./admin/ProjectsPanel";
import { ProvidersPanel } from "./admin/ProvidersPanel";
import { RawResourcesPanel } from "./admin/RawResourcesPanel";

/* The admin area (D-037): the catalog people build from, the projects, and who has access. Each
 * section explains itself, has a Docs button, and starts with something to do when it is empty. */

type Section = "modules" | "resources" | "providers" | "projects" | "people";

const NAV: { id: Section; label: string; hint: string; icon: ReactNode; group: string }[] = [
  { id: "modules", label: "Modules", hint: "Schemas people build from", icon: <Boxes className="size-4" />, group: "Catalog" },
  { id: "resources", label: "Raw resources", hint: "Plain provider blocks", icon: <Shapes className="size-4" />, group: "Catalog" },
  { id: "providers", label: "Cloud providers", hint: "Clouds, versions, logos", icon: <Cloud className="size-4" />, group: "Catalog" },
  { id: "projects", label: "Projects", hint: "Owners, repositories, review", icon: <FolderGit2 className="size-4" />, group: "Workspace" },
  { id: "people", label: "People & access", hint: "Sign-ups and GitHub", icon: <Users className="size-4" />, group: "Workspace" },
];

const TOPIC: Record<Section, HelpTopic> = { modules: "modules", resources: "resources", providers: "providers", projects: "projects", people: "people" };

export function AdminView({ onProjectsChanged }: { onProjectsChanged?: () => void }) {
  const [section, setSection] = useState<Section>("modules");
  const [denied, setDenied] = useState(false);
  // The documentation is a page of its own (not a drawer): null means the section itself.
  const [help, setHelp] = useState<HelpTopic | null>(null);
  const [waiting, setWaiting] = useState(0);
  const [exampleTick, setExampleTick] = useState(0);

  const onDenied = useCallback(() => setDenied(true), []);
  const onWaiting = useCallback((n: number) => setWaiting(n), []);

  if (denied) {
    return (
      <div className="p-8 text-sm text-[var(--tf-color-text-muted)]">
        The admin area needs the <strong>devops</strong> role.
      </div>
    );
  }

  const groups = [...new Set(NAV.map((n) => n.group))];

  return (
    <div className="flex min-h-0 flex-1 flex-col md:flex-row">
      <nav aria-label="Administration" className="shrink-0 border-b border-[var(--tf-color-border)] p-3 md:w-60 md:border-b-0 md:border-r md:p-4">
        <p className="m-0 mb-3 hidden px-2 text-xs font-semibold uppercase tracking-widest text-[var(--tf-color-text-muted)] md:block">Administration</p>
        <div className="flex gap-3 overflow-x-auto md:block md:space-y-5 md:overflow-visible">
          {groups.map((g) => (
            <div key={g} className="flex shrink-0 gap-1 md:block">
              <p className="m-0 mb-1 hidden px-2 text-[11px] font-semibold uppercase tracking-wider text-[var(--tf-color-text-muted)] md:block">{g}</p>
              {NAV.filter((n) => n.group === g).map((n) => (
                <button
                  key={n.id}
                  onClick={() => {
                    setHelp(null);
                    setSection(n.id);
                  }}
                  aria-current={section === n.id ? "page" : undefined}
                  className={`flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-left text-sm transition ${
                    section === n.id
                      ? "bg-[var(--tf-glass-strong)] font-semibold text-[var(--tf-color-text)]"
                      : "text-[var(--tf-color-text-secondary)] hover:bg-[var(--tf-glass-hover)]"
                  }`}
                >
                  <span className={section === n.id ? "text-[var(--tf-color-action)]" : ""}>{n.icon}</span>
                  <span className="min-w-0 flex-1">
                    <span className="block whitespace-nowrap">{n.label}</span>
                    <span className="hidden text-[11px] font-normal text-[var(--tf-color-text-muted)] md:block">{n.hint}</span>
                  </span>
                  {n.id === "people" && waiting > 0 && (
                    <span className="tf-badge" data-tone="warn" title={`${waiting} waiting for approval`}>
                      {waiting}
                    </span>
                  )}
                </button>
              ))}
            </div>
          ))}
        </div>
      </nav>

      <div className="min-w-0 flex-1 overflow-y-auto p-5 md:p-8">
        <div className={help ? "mx-auto max-w-6xl" : "mx-auto max-w-5xl"}>
          {help && (
            <DocsPage
              topic={help}
              onTopic={setHelp}
              onBack={() => setHelp(null)}
              onUseExample={() => {
                setHelp(null);
                setSection("modules");
                setExampleTick((n) => n + 1);
              }}
            />
          )}
          {!help && section === "modules" && <ModulesPanel onDenied={onDenied} onHelp={() => setHelp(TOPIC.modules)} exampleTick={exampleTick} />}
          {!help && section === "resources" && <RawResourcesPanel onDenied={onDenied} onHelp={() => setHelp(TOPIC.resources)} />}
          {!help && section === "providers" && <ProvidersPanel onDenied={onDenied} onHelp={() => setHelp(TOPIC.providers)} />}
          {!help && section === "projects" && <ProjectsPanel onDenied={onDenied} onChanged={onProjectsChanged} onHelp={() => setHelp(TOPIC.projects)} />}
          {!help && section === "people" && <PeoplePanel onDenied={onDenied} onHelp={() => setHelp(TOPIC.people)} onWaiting={onWaiting} />}
        </div>
      </div>

    </div>
  );
}
