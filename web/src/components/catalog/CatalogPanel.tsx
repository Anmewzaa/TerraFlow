import { useState } from "react";
import { useCatalog } from "../../hooks/useCatalog";
import type { ModuleSummary } from "../../types/catalog";
import { getAzureIcon, moduleDomainMap, domainBgOpacity, domainCssVars } from "../nodes/domainStyles";

interface Props {
  onSelect: (mod: ModuleSummary) => void;
  selected: string | null;
}

export function CatalogPanel({ onSelect, selected }: Props) {
  const { modules, loading } = useCatalog();
  const [search, setSearch] = useState("");

  const filtered = search
    ? modules.filter(
        (m) =>
          m.title.toLowerCase().includes(search.toLowerCase()) ||
          m.name.toLowerCase().includes(search.toLowerCase())
      )
    : modules;

  return (
    <aside className="tf-sidebar">
      <div className="tf-sidebar__header">
        <span className="tf-sidebar__title">Module Catalog</span>
        <div style={{ position: "relative" }}>
          <input
            type="text"
            placeholder="Search..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{
              width: 100,
              height: 24,
              padding: "0 var(--tf-space-2)",
              fontSize: 11,
              border: "1px solid var(--tf-color-border)",
              borderRadius: "var(--tf-radius-sm)",
              background: "var(--tf-color-surface-raised)",
              color: "var(--tf-color-text)",
              fontFamily: "inherit",
            }}
          />
        </div>
      </div>
      <div className="tf-sidebar__drag-hint">Drag a module onto the canvas</div>
      <div className="tf-sidebar__body">
        {loading && (
          <div style={{ padding: "var(--tf-space-4)", color: "var(--tf-color-text-muted)", textAlign: "center" }}>
            Loading...
          </div>
        )}
        {filtered.map((mod) => {
          const domain = moduleDomainMap[mod.name] ?? "neutral";
          const iconSrc = getAzureIcon(mod.name);
          const color = domainCssVars[domain] ?? domainCssVars.neutral;
          const iconBg = domainBgOpacity[domain] ?? domainBgOpacity.neutral;

          return (
            <button
              key={mod.name}
              className={`tf-catalog-item${selected === mod.name ? " tf-catalog-item--selected" : ""}`}
              onClick={() => onSelect(mod)}
              draggable
              onDragStart={(e) => {
                e.dataTransfer.setData(
                  "application/terraflow-module",
                  JSON.stringify({ name: mod.name, version: mod.latest_version, title: mod.title })
                );
                e.dataTransfer.effectAllowed = "copy";
              }}
            >
              <div className="tf-catalog-item__icon" style={{ background: iconBg, color }}>
                {iconSrc ? (
                  <img src={iconSrc} alt="" style={{ width: 22, height: 22 }} />
                ) : (
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                    <rect x="3" y="3" width="18" height="18" rx="2" />
                  </svg>
                )}
              </div>
              <div className="tf-catalog-item__info">
                <div className="tf-catalog-item__name">{mod.title}</div>
                <div className="tf-catalog-item__desc">{mod.description}</div>
                <div className="tf-catalog-item__version">
                  {mod.versions.join(", ")}
                </div>
              </div>
            </button>
          );
        })}
      </div>
    </aside>
  );
}
