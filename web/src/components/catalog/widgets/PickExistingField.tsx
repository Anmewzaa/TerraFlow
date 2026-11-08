import { useMemo } from "react";
import type { ProjectGraph, GraphNode } from "../../../types/graph";

interface Props {
  label: string;
  value: unknown;
  onChange: (value: string) => void;
  pick: { module_type: string; output: string };
  graph: ProjectGraph | null;
  disabled?: boolean;
}

export function PickExistingField({ label, value, onChange, pick, graph, disabled }: Props) {
  const options = useMemo(() => {
    if (!graph) return [];
    const matches: { id: string; name: string; folder: string }[] = [];
    for (const folder of graph.folders) {
      for (const node of folder.nodes) {
        if (node.kind === "module" && node.type === pick.module_type && node.name) {
          matches.push({ id: node.id, name: node.name, folder: folder.name });
        }
      }
    }
    return matches;
  }, [graph, pick.module_type]);

  return (
    <div style={{ marginBottom: "var(--tf-space-3)" }}>
      <label
        style={{
          display: "block",
          fontSize: "var(--tf-text-small)",
          fontWeight: 500,
          marginBottom: "var(--tf-space-1)",
          color: "var(--tf-color-text-secondary)",
        }}
      >
        {label}
      </label>
      <select
        value={typeof value === "string" ? value : ""}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        style={{
          width: "100%",
          padding: "var(--tf-space-2)",
          fontSize: "var(--tf-text-small)",
          fontFamily: "var(--tf-font-mono)",
          borderRadius: "var(--tf-radius-s)",
          border: "1px solid var(--tf-color-border)",
          background: disabled ? "var(--tf-color-canvas)" : "var(--tf-color-surface)",
          color: "var(--tf-color-text)",
        }}
      >
        <option value="">— Select existing resource —</option>
        {options.map((opt) => (
          <option key={opt.id} value={opt.name}>
            {opt.name} ({opt.folder}/)
          </option>
        ))}
      </select>
    </div>
  );
}
