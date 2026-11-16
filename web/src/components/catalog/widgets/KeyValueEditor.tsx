import { useEffect, useRef, useState } from "react";
import { X } from "lucide-react";
import { rowsToObject, toRows, withBlankRow, type KVRow } from "./rows";

interface Props {
  value: Record<string, string>;
  onChange: (value: unknown) => void;
}

const cell: React.CSSProperties = { fontFamily: "var(--tf-font-mono)" };
const removable: React.CSSProperties = { paddingRight: 30 };
const removeBtn: React.CSSProperties = {
  position: "absolute", right: 6, top: "50%", transform: "translateY(-50%)", display: "flex",
  background: "none", border: "none", padding: 2, cursor: "pointer", color: "var(--tf-color-text-muted)",
};

/**
 * Key / value pairs. There is always one empty row at the end: typing in it
 * adds the pair and opens the next empty row, so there is no Add button.
 */
export function KeyValueEditor({ value, onChange }: Props) {
  const [rows, setRows] = useState<KVRow[]>(() => withBlankRow(toRows(value)));
  const emitted = useRef(JSON.stringify(rowsToObject(rows)));

  // Follow a change that did not come from typing here (e.g. a reset).
  useEffect(() => {
    const incoming = JSON.stringify(value ?? {});
    if (incoming !== emitted.current) {
      emitted.current = incoming;
      setRows(withBlankRow(toRows(value)));
    }
  }, [value]);

  const commit = (next: KVRow[]) => {
    const full = withBlankRow(next);
    setRows(full);
    const obj = rowsToObject(full);
    emitted.current = JSON.stringify(obj);
    onChange(obj);
  };

  const edit = (id: number, patch: Partial<KVRow>) => commit(rows.map((r) => (r.id === id ? { ...r, ...patch } : r)));
  const remove = (id: number) => commit(rows.filter((r) => r.id !== id));

  const seen = new Map<string, number>();
  for (const r of rows) if (r.k.trim()) seen.set(r.k.trim(), (seen.get(r.k.trim()) ?? 0) + 1);

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
      {rows.map((r, i) => {
        const blank = i === rows.length - 1 && !r.k && !r.v;
        const dup = !!r.k.trim() && (seen.get(r.k.trim()) ?? 0) > 1;
        return (
          <div key={r.id} style={{ display: "grid", gridTemplateColumns: "minmax(0,1fr) minmax(0,1.3fr)", gap: 6 }}>
            <input
              aria-label="Key"
              placeholder={blank ? "New key" : "Key"}
              value={r.k}
              onChange={(e) => edit(r.id, { k: e.target.value })}
              title={dup ? "This key is used twice; the last one wins" : undefined}
              className="tf-input"
              style={dup ? { ...cell, borderColor: "var(--tf-color-error)" } : cell}
            />
            <div style={{ position: "relative" }}>
              <input
                aria-label="Value"
                placeholder={blank ? "Value" : ""}
                value={r.v}
                onChange={(e) => edit(r.id, { v: e.target.value })}
                className="tf-input"
                style={blank ? cell : { ...cell, ...removable }}
              />
              {!blank && (
                <button
                  type="button"
                  onClick={() => remove(r.id)}
                  aria-label={`Remove ${r.k || "row"}`}
                  title="Remove"
                  style={removeBtn}
                >
                  <X size={14} />
                </button>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}
