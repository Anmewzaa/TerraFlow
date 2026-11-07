import type { Project } from "../types/project";

interface ProjectSelectorProps {
  projects: Project[];
  selected: string | null;
  onSelect: (id: string) => void;
}

export function ProjectSelector({ projects, selected, onSelect }: ProjectSelectorProps) {
  return (
    <select
      value={selected ?? ""}
      onChange={(e) => onSelect(e.target.value)}
      style={{
        padding: "var(--tf-space-1) var(--tf-space-2)",
        borderRadius: "var(--tf-radius-s)",
        border: "1px solid var(--tf-color-border)",
        background: "var(--tf-color-surface)",
        color: "var(--tf-color-text)",
        fontSize: "var(--tf-text-small)",
        fontFamily: "var(--tf-font-sans)",
      }}
    >
      <option value="" disabled>Select project…</option>
      {projects.map((p) => (
        <option key={p.id} value={p.id}>{p.name}</option>
      ))}
    </select>
  );
}
