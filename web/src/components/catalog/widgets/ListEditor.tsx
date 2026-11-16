import { useEffect, useRef, useState } from "react";
import { X } from "lucide-react";
import { itemsToList, newId, withBlankItem, type ListRow } from "./rows";

interface Props {
  value: string[];
  onChange: (value: unknown) => void;
}

const cell: React.CSSProperties = { fontFamily: "var(--tf-font-mono)" };
const removable: React.CSSProperties = { paddingRight: 30 };
const removeBtn: React.CSSProperties = {
  position: "absolute", right: 6, top: "50%", transform: "translateY(-50%)", display: "flex",
  background: "none", border: "none", padding: 2, cursor: "pointer", color: "var(--tf-color-text-muted)",
};

const toRows = (items: string[]): ListRow[] => withBlankItem((items ?? []).map((text) => ({ id: newId(), text: String(text) })));

/** A list of text items. The empty row at the end becomes an item as soon as you type in it. */
export function ListEditor({ value, onChange }: Props) {
  const [rows, setRows] = useState<ListRow[]>(() => toRows(value));
  const emitted = useRef(JSON.stringify(itemsToList(rows)));

  useEffect(() => {
    const incoming = JSON.stringify(value ?? []);
    if (incoming !== emitted.current) {
      emitted.current = incoming;
      setRows(toRows(value));
    }
  }, [value]);

  const commit = (next: ListRow[]) => {
    const full = withBlankItem(next);
    setRows(full);
    const list = itemsToList(full);
    emitted.current = JSON.stringify(list);
    onChange(list);
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
      {rows.map((r, i) => {
        const blank = i === rows.length - 1 && !r.text;
        return (
          <div key={r.id} style={{ position: "relative" }}>
            <input
              aria-label={blank ? "New item" : `Item ${i + 1}`}
              placeholder={blank ? "Add an item" : ""}
              value={r.text}
              onChange={(e) => commit(rows.map((x) => (x.id === r.id ? { ...x, text: e.target.value } : x)))}
              className="tf-input"
              style={blank ? cell : { ...cell, ...removable }}
            />
            {!blank && (
              <button
                type="button"
                onClick={() => commit(rows.filter((x) => x.id !== r.id))}
                aria-label={`Remove item ${i + 1}`}
                title="Remove"
                style={removeBtn}
              >
                <X size={14} />
              </button>
            )}
          </div>
        );
      })}
    </div>
  );
}
