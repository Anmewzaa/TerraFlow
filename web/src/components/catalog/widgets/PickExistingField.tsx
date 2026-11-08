import { useMemo } from "react";
import { valueText } from "../../../lib/valueText";
import type { ProjectGraph } from "../../../types/graph";
import type { CanvasNode } from "../../../types/changeset";

interface Props {
  label: string;
  value: unknown;
  onChange: (value: string) => void;
  pick: { module_type: string; output: string };
  graph: ProjectGraph | null;
  draftNodes?: CanvasNode[];
  disabled?: boolean;
}

export function PickExistingField({ label, value, onChange, pick, graph, draftNodes, disabled }: Props) {
  const options = useMemo(() => {
    const matches: { id: string; name: string; folder: string; draft?: boolean }[] = [];

    if (graph) {
      for (const folder of graph.folders) {
        for (const node of folder.nodes) {
          if (node.kind === "module" && node.type === pick.module_type && node.name) {
            matches.push({ id: node.id, name: node.name, folder: folder.name });
          }
        }
      }
    }

    if (draftNodes) {
      for (const dn of draftNodes) {
        if (dn.module_name === pick.module_type && valueText(dn.values?.name)) {
          matches.push({
            id: dn.id,
            name: valueText(dn.values.name),
            folder: dn.target_folder,
            draft: true,
          });
        }
      }
    }

    return matches;
  }, [graph, draftNodes, pick.module_type]);

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
        disabled={disabled || options.length === 0}
        className="tf-input"
        style={{ fontFamily: "var(--tf-font-mono)" }}
      >
        <option value="">
          {options.length === 0 ? "No matching resource yet" : "Select existing resource"}
        </option>
        {options.map((opt) => (
          <option key={opt.id} value={opt.name}>
            {opt.name} ({opt.folder}/) {opt.draft ? "[draft]" : ""}
          </option>
        ))}
      </select>
      {options.length === 0 && (
        <div style={{ marginTop: 4, fontSize: "var(--tf-text-caption)", color: "var(--tf-color-text-muted)" }}>
          Add a {pick.module_type} to the canvas (or the repo) first, then pick it here.
        </div>
      )}
    </div>
  );
}
