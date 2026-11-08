import { useCatalog } from "../../hooks/useCatalog";
import type { ModuleSummary } from "../../types/catalog";

interface Props {
  onSelect: (mod: ModuleSummary) => void;
  selected: string | null;
}

export function CatalogPanel({ onSelect, selected }: Props) {
  const { modules, loading } = useCatalog();

  return (
    <div
      style={{
        width: 260,
        borderRight: "1px solid var(--tf-color-border)",
        background: "var(--tf-color-surface)",
        display: "flex",
        flexDirection: "column",
        flexShrink: 0,
      }}
    >
      <div
        style={{
          padding: "var(--tf-space-3) var(--tf-space-3)",
          borderBottom: "1px solid var(--tf-color-border)",
          fontWeight: 600,
          fontSize: "var(--tf-text-small)",
          color: "var(--tf-color-text-secondary)",
          textTransform: "uppercase",
          letterSpacing: "0.05em",
        }}
      >
        Module Catalog
      </div>
      <div style={{ flex: 1, overflow: "auto", padding: "var(--tf-space-2)" }}>
        {loading && (
          <div
            style={{
              padding: "var(--tf-space-4)",
              color: "var(--tf-color-text-muted)",
              textAlign: "center",
            }}
          >
            Loading…
          </div>
        )}
        {modules.map((mod) => (
          <button
            key={mod.name}
            onClick={() => onSelect(mod)}
            draggable
            onDragStart={(e) => {
              e.dataTransfer.setData(
                "application/terraflow-module",
                JSON.stringify({ name: mod.name, version: mod.latest_version, title: mod.title })
              );
              e.dataTransfer.effectAllowed = "copy";
            }}
            style={{
              display: "block",
              width: "100%",
              textAlign: "left",
              padding: "var(--tf-space-2) var(--tf-space-3)",
              marginBottom: "var(--tf-space-1)",
              border:
                selected === mod.name
                  ? "1px solid var(--tf-color-action)"
                  : "1px solid transparent",
              borderRadius: "var(--tf-radius-s)",
              background:
                selected === mod.name
                  ? "var(--tf-color-action-subtle)"
                  : "transparent",
              cursor: "grab",
              fontSize: "var(--tf-text-small)",
            }}
          >
            <div style={{ fontWeight: 500, color: "var(--tf-color-text)" }}>
              {mod.title}
            </div>
            <div
              style={{
                fontSize: "var(--tf-text-xs)",
                color: "var(--tf-color-text-muted)",
                marginTop: 2,
              }}
            >
              {mod.description}
            </div>
            <div
              style={{
                fontSize: "var(--tf-text-xs)",
                color: "var(--tf-color-text-muted)",
                marginTop: 2,
              }}
            >
              v: {mod.versions.join(", ")}
            </div>
          </button>
        ))}
      </div>
    </div>
  );
}
