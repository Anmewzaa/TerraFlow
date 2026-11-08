import { useMemo, useState } from "react";
import { GripVertical, Loader2, PackageSearch, Search, X } from "lucide-react";
import { useCatalog } from "../../hooks/useCatalog";
import type { ModuleSummary } from "../../types/catalog";
import { getAzureIcon, moduleDomainMap, domainBgOpacity, domainCssVars } from "../nodes/domainStyles";

interface Props {
  onSelect: (mod: ModuleSummary) => void;
  selected: string | null;
}

const DOMAIN_LABEL: Record<string, string> = {
  network: "Networking",
  compute: "Compute",
  data: "Data & Storage",
  security: "Security",
  neutral: "General",
};
const DOMAIN_ORDER = ["network", "compute", "data", "security", "neutral"];

export function CatalogPanel({ onSelect, selected }: Props) {
  const { modules, loading } = useCatalog();
  const [search, setSearch] = useState("");

  const groups = useMemo(() => {
    const q = search.trim().toLowerCase();
    const filtered = q
      ? modules.filter(
          (m) =>
            m.title.toLowerCase().includes(q) ||
            m.name.toLowerCase().includes(q) ||
            (m.description ?? "").toLowerCase().includes(q),
        )
      : modules;
    const byDomain = new Map<string, ModuleSummary[]>();
    for (const m of filtered) {
      const d = moduleDomainMap[m.name] ?? "neutral";
      byDomain.set(d, [...(byDomain.get(d) ?? []), m]);
    }
    return DOMAIN_ORDER.filter((d) => byDomain.has(d)).map((d) => ({ domain: d, items: byDomain.get(d)! }));
  }, [modules, search]);

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <div className="space-y-3 border-b border-[var(--tf-color-border)] p-4">
        <div className="flex items-baseline justify-between">
          <h2 className="text-sm font-semibold text-[var(--tf-color-text)]">Module catalog</h2>
          <span className="text-xs text-[var(--tf-color-text-muted)]">{modules.length} modules</span>
        </div>
        <div className="relative">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-[var(--tf-color-text-muted)]" />
          <input
            type="text"
            placeholder="Search modules…"
            aria-label="Search modules"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="h-9 w-full rounded-lg border border-[var(--tf-color-border)] bg-[var(--tf-color-surface-raised)] pl-8 pr-8 text-sm text-[var(--tf-color-text)] outline-none transition focus:border-[var(--tf-color-action)]"
          />
          {search && (
            <button
              onClick={() => setSearch("")}
              aria-label="Clear search"
              className="absolute right-2 top-1/2 -translate-y-1/2 text-[var(--tf-color-text-muted)] hover:text-[var(--tf-color-text)]"
            >
              <X className="size-4" />
            </button>
          )}
        </div>
        <p className="rounded-md border border-dashed border-[var(--tf-color-border)] px-3 py-1.5 text-center text-[11px] text-[var(--tf-color-text-muted)]">
          Drag a module onto the canvas
        </p>
      </div>

      <div className="flex-1 space-y-8 overflow-y-auto p-3 pb-10">
        {loading && (
          <div className="flex items-center justify-center gap-2 py-8 text-sm text-[var(--tf-color-text-muted)]">
            <Loader2 className="size-4 animate-spin" /> Loading…
          </div>
        )}
        {!loading && groups.length === 0 && (
          <div className="flex flex-col items-center gap-2 py-10 text-center text-sm text-[var(--tf-color-text-muted)]">
            <PackageSearch className="size-8" />
            {search ? `No modules match “${search}”` : "No modules available"}
          </div>
        )}
        {groups.map(({ domain, items }) => {
          const color = domainCssVars[domain] ?? domainCssVars.neutral;
          return (
            <section key={domain}>
              <h3 className="mb-3 flex items-center gap-2 px-1 text-[11px] font-semibold uppercase tracking-wider text-[var(--tf-color-text-muted)]">
                <span className="size-2 rounded-sm" style={{ background: color }} />
                {DOMAIN_LABEL[domain]}
                <span className="font-normal">({items.length})</span>
              </h3>
              <div className="space-y-2.5">
                {items.map((mod) => {
                  const iconSrc = getAzureIcon(mod.name);
                  const isSel = selected === mod.name;
                  return (
                    <button
                      key={mod.name}
                      onClick={() => onSelect(mod)}
                      draggable
                      onDragStart={(e) => {
                        e.dataTransfer.setData(
                          "application/terraflow-module",
                          JSON.stringify({ name: mod.name, version: mod.latest_version, title: mod.title }),
                        );
                        e.dataTransfer.effectAllowed = "copy";
                      }}
                      className={`group flex w-full cursor-grab items-center gap-3 rounded-xl border p-2.5 text-left transition active:cursor-grabbing ${
                        isSel
                          ? "border-[var(--tf-color-action)] bg-[var(--tf-color-surface-selected)]"
                          : "border-[var(--tf-color-border)] bg-[var(--tf-color-surface-raised)] hover:-translate-y-px hover:border-[var(--tf-color-border-strong)] hover:shadow-md"
                      }`}
                    >
                      <div
                        className="flex size-10 shrink-0 items-center justify-center rounded-lg"
                        style={{ background: domainBgOpacity[domain] ?? domainBgOpacity.neutral, color }}
                      >
                        {iconSrc ? (
                          <img src={iconSrc} alt="" className="size-6" />
                        ) : (
                          <PackageSearch className="size-5" />
                        )}
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="truncate text-[13px] font-medium text-[var(--tf-color-text)]">{mod.title}</div>
                        <div className="truncate text-[11px] text-[var(--tf-color-text-muted)]">{mod.description}</div>
                        <div className="mt-1 flex gap-1 overflow-hidden">
                          {mod.versions.slice(0, 3).map((v) => (
                            <span
                              key={v}
                              className="rounded bg-[var(--tf-color-surface-subtle)] px-1.5 py-px font-mono text-[10px] text-[var(--tf-color-text-secondary)]"
                            >
                              {v}
                            </span>
                          ))}
                        </div>
                      </div>
                      <GripVertical className="size-4 shrink-0 text-[var(--tf-color-text-muted)] opacity-0 transition group-hover:opacity-100" />
                    </button>
                  );
                })}
              </div>
            </section>
          );
        })}
      </div>
    </div>
  );
}
