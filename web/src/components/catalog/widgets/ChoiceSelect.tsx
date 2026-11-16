import { useState } from "react";
import type { ChoiceSet } from "../../../lib/options";

const OTHER = "__other__";

interface Props {
  value: string;
  set: ChoiceSet;
  label: string;
  onChange: (value: string | undefined) => void;
}

/** A dropdown whose choices come from the schema. A value that is not listed is kept and shown, never lost. */
export function ChoiceSelect({ value, set, label, onChange }: Props) {
  const listed = set.choices.some((c) => c.value === value);
  const [custom, setCustom] = useState(!!value && !listed);
  const showText = set.allowCustom && custom;
  return (
    <div style={{ display: "grid", gap: 6 }}>
      <select
        aria-label={label}
        className="tf-input"
        value={showText ? OTHER : value}
        onChange={(e) => {
          if (e.target.value === OTHER) {
            setCustom(true);
            return;
          }
          setCustom(false);
          onChange(e.target.value || undefined);
        }}
      >
        <option value="">Choose...</option>
        {!listed && value && !set.allowCustom && <option value={value}>{value} (not in the list)</option>}
        {set.choices.map((c) => (
          <option key={c.value} value={c.value}>
            {c.label}
          </option>
        ))}
        {set.allowCustom && <option value={OTHER}>Other...</option>}
      </select>
      {showText && (
        <input
          type="text"
          aria-label={`${label} (other)`}
          className="tf-input"
          value={listed ? "" : value}
          placeholder="Type the value"
          onChange={(e) => onChange(e.target.value || undefined)}
        />
      )}
    </div>
  );
}
