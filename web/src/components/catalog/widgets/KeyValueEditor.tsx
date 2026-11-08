import { useState } from "react";

interface Props {
  value: Record<string, string>;
  onChange: (value: unknown) => void;
}

export function KeyValueEditor({ value, onChange }: Props) {
  const [newKey, setNewKey] = useState("");
  const [newVal, setNewVal] = useState("");

  const entries = Object.entries(value);

  const addEntry = () => {
    if (!newKey.trim()) return;
    onChange({ ...value, [newKey.trim()]: newVal });
    setNewKey("");
    setNewVal("");
  };

  const removeEntry = (key: string) => {
    const next = { ...value };
    delete next[key];
    onChange(next);
  };

  const inputStyle: React.CSSProperties = {
    padding: "var(--tf-space-1) var(--tf-space-2)",
    fontSize: "var(--tf-text-xs)",
    border: "1px solid var(--tf-color-border)",
    borderRadius: "var(--tf-radius-s)",
    background: "var(--tf-color-surface)",
    color: "var(--tf-color-text)",
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--tf-space-1)" }}>
      {entries.map(([k, v]) => (
        <div key={k} style={{ display: "flex", alignItems: "center", gap: "var(--tf-space-1)" }}>
          <span style={{ fontSize: "var(--tf-text-xs)", fontWeight: 500, minWidth: 80 }}>{k}</span>
          <span style={{ fontSize: "var(--tf-text-xs)", color: "var(--tf-color-text-muted)" }}>{v}</span>
          <button
            onClick={() => removeEntry(k)}
            style={{
              marginLeft: "auto",
              background: "none",
              border: "none",
              color: "var(--tf-color-bad)",
              cursor: "pointer",
              fontSize: "var(--tf-text-xs)",
            }}
          >
            x
          </button>
        </div>
      ))}
      <div style={{ display: "flex", gap: "var(--tf-space-1)", marginTop: "var(--tf-space-1)" }}>
        <input
          type="text"
          placeholder="Key"
          value={newKey}
          onChange={(e) => setNewKey(e.target.value)}
          style={{ ...inputStyle, flex: 1 }}
        />
        <input
          type="text"
          placeholder="Value"
          value={newVal}
          onChange={(e) => setNewVal(e.target.value)}
          style={{ ...inputStyle, flex: 1 }}
        />
        <button
          onClick={addEntry}
          style={{
            padding: "var(--tf-space-1) var(--tf-space-2)",
            fontSize: "var(--tf-text-xs)",
            background: "var(--tf-color-action)",
            color: "#fff",
            border: "none",
            borderRadius: "var(--tf-radius-s)",
            cursor: "pointer",
          }}
        >
          Add
        </button>
      </div>
    </div>
  );
}
