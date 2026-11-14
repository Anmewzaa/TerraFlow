import { useMemo, useState } from "react";
import { Search, X } from "lucide-react";
import { DriftPanel } from "./DriftPanel";
import type { ProjectGraph } from "../types/graph";
import type { CanvasNode } from "../types/changeset";

interface Props {
  graph: ProjectGraph | null;
  drafts: CanvasNode[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  /** Repository drift is shown above the list when the project and environment are known (8.14). */
  drift?: { projectId: string; env: string; canManage: boolean };
}

/** The module calls already in the repository, by folder, with search. */
export function ResourcesPanel({ graph, drafts, selectedId, onSelect, drift }: Props) {
  const [search, setSearch] = useState("");
  const editing = useMemo(() => new Set(drafts.filter((d) => d.existing).map((d) => d.id)), [drafts]);

  const groups = useMemo(() => {
    const q = search.trim().toLowerCase();
    return (graph?.folders ?? [])
      .map((f) => ({
        folder: f.name,
        items: f.nodes
          .filter((n) => n.kind === "module")
          .filter((n) => !q || `${n.label ?? ""} ${n.name ?? ""} ${n.type} ${f.name}`.toLowerCase().includes(q)),
      }))
      .filter((g) => g.items.length > 0);
  }, [graph, search]);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {drift && <DriftPanel projectId={drift.projectId} env={drift.env} canManage={drift.canManage} onOpenNode={onSelect} />}
      <div className="border-b border-[var(--tf-color-border)] p-3">
        <div className="relative">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-[var(--tf-color-text-muted)]" />
          <input
            type="text"
            placeholder="Search resources…"
            aria-label="Search resources"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="h-9 w-full rounded-lg border border-[var(--tf-color-border)] bg-[var(--tf-color-surface-raised)] pl-8 pr-8 text-sm text-[var(--tf-color-text)] outline-none focus:border-[var(--tf-color-action)]"
          />
          {search && (
            <button
              onClick={() => setSearch("")}
              aria-label="Clear search"
              className="absolute right-2 top-1/2 -translate-y-1/2 text-[var(--tf-color-text-muted)]"
            >
              <X className="size-4" />
            </button>
          )}
        </div>
      </div>
      <div className="flex-1 space-y-8 overflow-y-auto p-3 pb-10">
        {groups.length === 0 && (
          <p className="py-8 text-center text-sm text-[var(--tf-color-text-muted)]">
            {search ? `No resources match “${search}”` : "No resources in the repository yet"}
          </p>
        )}
        {groups.map((g) => (
          <section key={g.folder}>
            <h3 className="mb-3 px-1 text-[11px] font-semibold uppercase tracking-wider text-[var(--tf-color-text-muted)]">
              {g.folder} <span className="font-normal">({g.items.length})</span>
            </h3>
            <ul className="m-0 list-none space-y-2 p-0">
              {g.items.map((n) => (
                <li key={n.id}>
                  <button
                    onClick={() => onSelect(n.id)}
                    className={`flex w-full items-center justify-between gap-2 rounded-lg border px-2.5 py-2 text-left text-[13px] ${
                      selectedId === n.id
                        ? "border-[var(--tf-color-action)] bg-[var(--tf-color-surface-selected)]"
                        : "border-[var(--tf-color-border)] bg-[var(--tf-color-surface-raised)] hover:border-[var(--tf-color-border-strong)]"
                    }`}
                  >
                    <span className="min-w-0">
                      <span className="block truncate font-medium text-[var(--tf-color-text)]">
                        {n.label ?? n.name ?? n.type}
                      </span>
                      <span className="block truncate text-[11px] text-[var(--tf-color-text-muted)]">{n.type}</span>
                    </span>
                    {editing.has(n.id) && (
                      <span className="shrink-0 rounded bg-[var(--tf-color-surface-subtle)] px-1.5 py-px text-[10px]">
                        editing
                      </span>
                    )}
                  </button>
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
    </div>
  );
}
