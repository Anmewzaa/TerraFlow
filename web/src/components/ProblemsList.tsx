import { useState } from "react";
import { AlertTriangle, ChevronDown, ChevronUp, CircleAlert } from "lucide-react";
import type { Finding } from "../lib/findings";
import { displayName } from "../lib/valueText";
import type { CanvasNode } from "../types/changeset";

interface Props {
  findings: Finding[];
  nodes: CanvasNode[];
  onSelect: (nodeId: string) => void;
}

/** Everything the server found on the canvas: what must be fixed first, then the project's own rules. Click a row to open the resource. */
export function ProblemsList({ findings, nodes, onSelect }: Props) {
  const [open, setOpen] = useState(false);
  const shown = findings.filter((f) => f.severity === "error" || f.rule === "policy");
  if (shown.length === 0) return null;
  const errors = shown.filter((f) => f.severity === "error");
  const rows = [...errors, ...shown.filter((f) => f.severity !== "error")];
  const nameOf = (id?: string) => {
    const n = nodes.find((x) => x.id === id);
    return n ? displayName(n.values, n.instance_name) : "the canvas";
  };
  return (
    <section
      aria-label="Problems"
      className="absolute bottom-16 right-3 z-10 w-[22rem] md:bottom-3 max-w-[calc(100%-1.5rem)] rounded-xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] shadow-lg"
    >
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
        className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm font-semibold"
      >
        {errors.length > 0 ? <CircleAlert className="size-4 text-[var(--tf-color-error)]" /> : <AlertTriangle className="size-4 text-[var(--tf-color-warning)]" />}
        <span className="flex-1">
          {errors.length > 0 && `${errors.length} to fix`}
          {errors.length > 0 && shown.length > errors.length && " · "}
          {shown.length > errors.length && `${shown.length - errors.length} to review`}
        </span>
        {open ? <ChevronDown className="size-4" /> : <ChevronUp className="size-4" />}
      </button>
      {open && (
        <ul className="m-0 max-h-60 list-none overflow-y-auto border-t border-[var(--tf-color-border)] p-0">
          {rows.map((f, i) => (
            <li key={`${f.node}-${f.rule}-${i}`}>
              <button
                type="button"
                disabled={!f.node}
                onClick={() => f.node && onSelect(f.node)}
                className="flex w-full gap-2 px-3 py-2 text-left text-xs hover:bg-[var(--tf-glass-hover)]"
              >
                <span className={f.severity === "error" ? "text-[var(--tf-color-error)]" : "text-[var(--tf-color-warning)]"}>
                  {f.severity === "error" ? "Fix" : "Review"}
                </span>
                <span className="min-w-0 flex-1">
                  <strong>{nameOf(f.node)}</strong>: {f.message}
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
