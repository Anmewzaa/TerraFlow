import { useState } from "react";
import type { CanvasNode } from "../types/changeset";
import { parseValues, rawIssues } from "../lib/rawResource";

interface Props {
  node: CanvasNode;
  folders: string[];
  onChange: (updates: Partial<CanvasNode>) => void;
}

const input =
  "mt-1 block w-full rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-canvas)] px-3 py-2 text-sm outline-none focus:border-[var(--tf-color-action)]";

/** Free-form editor for a raw provider resource (no catalog schema). */
export function RawResourceForm({ node, folders, onChange }: Props) {
  const [text, setText] = useState(() => JSON.stringify(node.values ?? {}, null, 2));
  const [parseError, setParseError] = useState<string | null>(null);
  const issues = rawIssues(node);
  const locked = !!node.existing; // an existing block keeps its type and place

  const onText = (t: string) => {
    setText(t);
    const r = parseValues(t);
    if ("error" in r) setParseError(r.error);
    else {
      setParseError(null);
      onChange({ values: r.values });
    }
  };

  return (
    <div className="grid gap-3 text-sm">
      <p className="m-0 text-xs text-[var(--tf-color-text-muted)]">
        A raw resource prepared by an admin. It is written as a <code>resource</code> block, not a module call, and
        has no schema, so its arguments are free-form and are not checked against the provider until the pipeline runs.
      </p>
      {issues.length > 0 && (
        <ul role="alert" className="m-0 list-disc pl-5 text-xs text-[var(--tf-color-error)]">
          {issues.map((i) => <li key={i.text}>{i.text}</li>)}
        </ul>
      )}
      <label>
        Resource type
        <input className={`${input} font-mono`} value={node.resource_type ?? ""} readOnly disabled />
      </label>
      <label>
        Name
        <input
          className={`${input} font-mono`}
          value={node.instance_name}
          placeholder="rg_main"
          onChange={(e) => onChange({ instance_name: e.target.value.trim() })}
        />
      </label>
      <div className="grid grid-cols-2 gap-3">
        <label>
          Folder
          <select
            className={input}
            value={node.target_folder}
            disabled={locked}
            onChange={(e) => onChange({ target_folder: e.target.value })}
          >
            {!folders.includes(node.target_folder) && <option value={node.target_folder}>{node.target_folder || "—"}</option>}
            {folders.map((f) => <option key={f} value={f}>{f}</option>)}
          </select>
        </label>
        <label>
          File
          <input
            className={`${input} font-mono`}
            value={node.filename ?? ""}
            disabled={locked}
            placeholder="main.tf"
            onChange={(e) => onChange({ filename: e.target.value.trim() || undefined })}
          />
        </label>
      </div>
      <label>
        Arguments (JSON)
        <textarea
          className={`${input} font-mono text-xs`}
          rows={12}
          spellCheck={false}
          value={text}
          onChange={(e) => onText(e.target.value)}
        />
      </label>
      {parseError ? (
        <p role="alert" className="m-0 text-xs text-[var(--tf-color-error)]">Not saved yet: {parseError}</p>
      ) : (
        <p className="m-0 text-xs text-[var(--tf-color-text-muted)]">
          Use <code>{'{"$expr": "var.location"}'}</code> for a reference and{" "}
          <code>{'{"ip_configuration": {"$blocks": [{...}]}}'}</code> for nested blocks.
        </p>
      )}
    </div>
  );
}
