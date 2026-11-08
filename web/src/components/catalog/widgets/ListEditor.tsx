import { useState } from "react";

interface Props {
  value: string[];
  onChange: (value: unknown) => void;
}

export function ListEditor({ value, onChange }: Props) {
  const [newItem, setNewItem] = useState("");

  const addItem = () => {
    if (!newItem.trim()) return;
    onChange([...value, newItem.trim()]);
    setNewItem("");
  };

  const removeItem = (index: number) => {
    onChange(value.filter((_, i) => i !== index));
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
      {value.map((item, i) => (
        <div key={i} style={{ display: "flex", alignItems: "center", gap: "var(--tf-space-1)" }}>
          <span style={{ fontSize: "var(--tf-text-xs)", flex: 1 }}>{item}</span>
          <button
            onClick={() => removeItem(i)}
            style={{
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
          placeholder="Add item…"
          value={newItem}
          onChange={(e) => setNewItem(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && addItem()}
          style={{ ...inputStyle, flex: 1 }}
        />
        <button
          onClick={addItem}
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
