import { useMemo, useState } from "react";
import { GripVertical, Loader2, PackageSearch, Search, X } from "lucide-react";
import type { ModuleSummary } from "../../types/catalog";
import type { RawResourceTemplate } from "../../hooks/useRawResources";
import type { Category, ProjectProvider, RegisteredProvider } from "../../lib/provider";
import { providerOfType } from "../../lib/provider";
import { categoryOfModule, groupByCategory, paletteModules, paletteRaw, usedCategories } from "../../lib/palette";
import { domainBgOpacity, domainCssVars, getAzureIcon, moduleDomainMap } from "../nodes/domainStyles";
import { ProviderLogo } from "../admin/ProvidersPanel";

interface Props {
  modules: ModuleSummary[];
  loading: boolean;
  error: string | null;
  rawTemplates: RawResourceTemplate[];
  projectProviders: ProjectProvider[] | undefined;
  registry: RegisteredProvider[];
  categories: Category[];
  selected: string | null;
  disabled: boolean;
  onSelect: (mod: ModuleSummary) => void;
  onAddRaw: (t: RawResourceTemplate) => void;
}

/**
 * The left palette: one search box, category chips, the project's providers, and two lists, modules
 * (each with its own version) and raw resources (only those that fit the project's provider and version).
 */
export function Palette({ modules, loading, error, rawTemplates, projectProviders, registry, categories, selected, disabled, onSelect, onAddRaw }: Props) {
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("");
  // The version chosen for a module; absent means its latest.
  const [versionOf, setVersionOf] = useState<Record<string, string>>({});

  const filter = useMemo(() => ({ query, category, providers: projectProviders }), [query, category, projectProviders]);
  const mods = useMemo(() => paletteModules(modules, filter), [modules, filter]);
  const raws = useMemo(() => paletteRaw(rawTemplates, filter), [rawTemplates, filter]);
  const chips = useMemo(() => usedCategories(modules, rawTemplates, categories), [modules, rawTemplates, categories]);
  const moduleGroups = useMemo(() => groupByCategory(mods.shown, categoryOfModule, categories), [mods.shown, categories]);
  const rawGroups = useMemo(() => groupByCategory(raws.shown, (t) => t.category || "other", categories), [raws.shown, categories]);
  const chosen = (projectProviders ?? []).map((p) => registry.find((r) => r.id === p.id)).filter((p): p is RegisteredProvider => !!p);
  const hidden = mods.hiddenByProvider + raws.hidden;
  const nothing = !loading && !error && mods.shown.length === 0 && raws.shown.length === 0;

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <div className="space-y-3 border-b border-[var(--tf-color-border)] p-4">
        <div className="flex items-center justify-between gap-2">
          <h2 className="m-0 text-sm font-semibold">Palette</h2>
          {chosen.length > 0 ? (
            <span className="flex items-center gap-1.5" aria-label="Providers of this project">
              {chosen.map((p) => (
                <span key={p.id} className="flex items-center gap-1 rounded-full bg-[var(--tf-color-surface-subtle)] py-0.5 pl-1 pr-2 text-[11px]" title={`${p.name} ${(projectProviders ?? []).find((x) => x.id === p.id)?.version ?? ""}`}>
                  <ProviderLogo provider={p} size={14} />
                  {(projectProviders ?? []).find((x) => x.id === p.id)?.version || p.id}
                </span>
              ))}
            </span>
          ) : (
            <span className="text-xs text-[var(--tf-color-text-muted)]">all providers</span>
          )}
        </div>
        <div className="relative">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-[var(--tf-color-text-muted)]" />
          <input
            type="text"
            placeholder="Search modules and resources…"
            aria-label="Search the palette"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="h-9 w-full rounded-lg border border-[var(--tf-color-border)] bg-[var(--tf-color-surface-raised)] pl-8 pr-8 text-sm text-[var(--tf-color-text)] outline-none transition focus:border-[var(--tf-color-action)]"
          />
          {query && (
            <button onClick={() => setQuery("")} aria-label="Clear search" className="absolute right-2 top-1/2 -translate-y-1/2 text-[var(--tf-color-text-muted)] hover:text-[var(--tf-color-text)]">
              <X className="size-4" />
            </button>
          )}
        </div>
        {chips.length > 1 && (
          <div role="tablist" aria-label="Category" className="flex flex-wrap gap-1.5">
            {[{ id: "", label: "All" }, ...chips].map((c) => (
              <button
                key={c.id}
                role="tab"
                aria-selected={category === c.id}
                onClick={() => setCategory(c.id)}
                className={`rounded-full border px-2.5 py-0.5 text-[11px] font-medium transition ${
                  category === c.id
                    ? "border-[var(--tf-color-action)] bg-[var(--tf-color-action)] text-[var(--tf-color-on-action)]"
                    : "border-[var(--tf-color-border)] text-[var(--tf-color-text-secondary)] hover:border-[var(--tf-color-action)]"
                }`}
              >
                {c.label}
              </button>
            ))}
          </div>
        )}
      </div>

      <div className="flex-1 space-y-6 overflow-y-auto p-3 pb-10">
        {loading && (
          <div className="flex items-center justify-center gap-2 py-8 text-sm text-[var(--tf-color-text-muted)]">
            <Loader2 className="size-4 animate-spin" /> Loading…
          </div>
        )}
        {error && <p role="alert" className="m-3 text-sm text-[var(--tf-color-error)]">Could not load the module catalog: {error}</p>}
        {nothing && (
          <div className="flex flex-col items-center gap-2 py-10 text-center text-sm text-[var(--tf-color-text-muted)]">
            <PackageSearch className="size-8" />
            {query ? `Nothing matches “${query}”` : "Nothing to show here yet"}
          </div>
        )}

        {moduleGroups.length > 0 && (
          <section aria-label="Modules">
            <h3 className="mb-2 px-1 text-xs font-semibold">Modules <span className="font-normal text-[var(--tf-color-text-muted)]">({mods.shown.length}) · drag onto the canvas</span></h3>
            {moduleGroups.map(({ category: cat, items }) => (
              <div key={cat.id} className="mb-4">
                <h4 className="mb-1.5 px-1 text-[11px] font-semibold uppercase tracking-wider text-[var(--tf-color-text-muted)]">{cat.label}</h4>
                <div className="space-y-2">
                  {items.map((mod) => {
                    const domain = moduleDomainMap[mod.name] ?? "neutral";
                    const color = domainCssVars[domain] ?? domainCssVars.neutral;
                    const iconSrc = getAzureIcon(mod.name);
                    const version = versionOf[mod.name] ?? mod.latest_version;
                    return (
                      <div
                        key={mod.name}
                        role="button"
                        tabIndex={0}
                        draggable
                        onClick={() => onSelect(mod)}
                        onKeyDown={(e) => e.key === "Enter" && onSelect(mod)}
                        onDragStart={(e) => {
                          e.dataTransfer.setData("application/terraflow-module", JSON.stringify({ name: mod.name, version, title: mod.title }));
                          e.dataTransfer.effectAllowed = "copy";
                        }}
                        className={`group flex w-full cursor-grab items-center gap-3 rounded-xl border p-2.5 text-left transition active:cursor-grabbing ${
                          selected === mod.name
                            ? "border-[var(--tf-color-action)] bg-[var(--tf-color-surface-selected)]"
                            : "border-[var(--tf-color-border)] bg-[var(--tf-color-surface-raised)] hover:-translate-y-px hover:border-[var(--tf-color-border-strong)] hover:shadow-md"
                        }`}
                      >
                        <div className="flex size-10 shrink-0 items-center justify-center rounded-lg" style={{ background: domainBgOpacity[domain] ?? domainBgOpacity.neutral, color }}>
                          {iconSrc ? <img src={iconSrc} alt="" className="size-6" /> : <PackageSearch className="size-5" />}
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="truncate text-[13px] font-medium">
                            {mod.title}
                            {mod.scope === "project" && (
                              <span className="ml-1.5 rounded bg-[var(--tf-color-surface-subtle)] px-1 py-px align-middle text-[10px] font-normal text-[var(--tf-color-text-secondary)]" title={mod.shadows_global ? "This project's own. It is used here instead of the global one of the same name." : "This project's own"}>
                                project
                              </span>
                            )}
                          </div>
                          <div className="truncate text-[11px] text-[var(--tf-color-text-muted)]">{mod.description}</div>
                        </div>
                        {mod.versions.length > 1 ? (
                          <select
                            aria-label={`Version of ${mod.title}`}
                            value={version}
                            onClick={(e) => e.stopPropagation()}
                            onMouseDown={(e) => e.stopPropagation()}
                            onChange={(e) => setVersionOf({ ...versionOf, [mod.name]: e.target.value })}
                            className="shrink-0 rounded border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] px-1 py-0.5 font-mono text-[10px]"
                          >
                            {[...mod.versions].reverse().map((v) => (
                              <option key={v} value={v}>
                                {v}
                              </option>
                            ))}
                          </select>
                        ) : (
                          <span className="shrink-0 rounded bg-[var(--tf-color-surface-subtle)] px-1.5 py-px font-mono text-[10px] text-[var(--tf-color-text-secondary)]">{version}</span>
                        )}
                        <GripVertical className="size-4 shrink-0 text-[var(--tf-color-text-muted)] opacity-0 transition group-hover:opacity-100" />
                      </div>
                    );
                  })}
                </div>
              </div>
            ))}
          </section>
        )}

        {rawGroups.length > 0 && (
          <section aria-label="Raw resources">
            <h3 className="mb-2 px-1 text-xs font-semibold">Raw resources <span className="font-normal text-[var(--tf-color-text-muted)]">({raws.shown.length}) · click to add</span></h3>
            {rawGroups.map(({ category: cat, items }) => (
              <div key={cat.id} className="mb-4">
                <h4 className="mb-1.5 px-1 text-[11px] font-semibold uppercase tracking-wider text-[var(--tf-color-text-muted)]">{cat.label}</h4>
                <ul className="m-0 list-none space-y-2 p-0">
                  {items.map((t) => {
                    const prov = registry.find((p) => p.id === (t.provider || providerOfType(t.resource_type)));
                    return (
                      <li key={t.name}>
                        <button
                          disabled={disabled}
                          onClick={() => onAddRaw(t)}
                          title={t.description || `Add ${t.resource_type}`}
                          className="flex w-full items-center gap-2.5 rounded-lg border border-dashed border-[var(--tf-color-border-strong)] bg-[var(--tf-color-surface-raised)] px-2.5 py-2 text-left hover:border-[var(--tf-color-action)] disabled:opacity-50"
                        >
                          {prov && <ProviderLogo provider={prov} size={20} />}
                          <span className="min-w-0">
                            <span className="block truncate text-[13px] font-medium">
                              {t.title}
                              {t.scope === "project" && (
                              <span className="ml-1.5 rounded bg-[var(--tf-color-surface-subtle)] px-1 py-px align-middle text-[10px] font-normal text-[var(--tf-color-text-secondary)]" title={t.shadows_global ? "This project's own. It is used here instead of the global one of the same name." : "This project's own"}>
                                project
                              </span>
                            )}
                            </span>
                            <span className="block truncate font-mono text-[11px] text-[var(--tf-color-text-muted)]">{t.resource_type}</span>
                          </span>
                        </button>
                      </li>
                    );
                  })}
                </ul>
              </div>
            ))}
          </section>
        )}

        {hidden > 0 && (
          <p className="m-0 px-1 text-[11px] text-[var(--tf-color-text-muted)]">
            {hidden} {hidden === 1 ? "item is" : "items are"} hidden because {hidden === 1 ? "it belongs" : "they belong"} to a provider or version this project does not use (Settings, Cloud providers).
          </p>
        )}
      </div>
    </div>
  );
}
