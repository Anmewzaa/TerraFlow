import type { JSONSchema, FieldGroup, TFUIMetadata } from "../../types/catalog";
import { KeyValueEditor } from "./widgets/KeyValueEditor";
import { ListEditor } from "./widgets/ListEditor";
import { LockedField } from "./widgets/LockedField";

interface Props {
  schema: JSONSchema;
  values: Record<string, unknown>;
  onChange: (values: Record<string, unknown>) => void;
}

export function SchemaForm({ schema, values, onChange }: Props) {
  const groups = schema["x-tf-ui"]?.groups ?? [];
  const properties = schema.properties ?? {};
  const required = new Set(schema.required ?? []);

  const sorted = [...groups].sort((a, b) => a.order - b.order);

  const fieldsByGroup = new Map<string, string[]>();
  const ungrouped: string[] = [];

  for (const [key, prop] of Object.entries(properties)) {
    const ui = prop["x-tf-ui"];
    if (ui?.hidden) continue;
    const g = ui?.group;
    if (g) {
      if (!fieldsByGroup.has(g)) fieldsByGroup.set(g, []);
      fieldsByGroup.get(g)!.push(key);
    } else {
      ungrouped.push(key);
    }
  }

  for (const fields of fieldsByGroup.values()) {
    fields.sort((a, b) => {
      const oa = properties[a]["x-tf-ui"]?.order ?? 999;
      const ob = properties[b]["x-tf-ui"]?.order ?? 999;
      return oa - ob;
    });
  }

  const setField = (key: string, value: unknown) => {
    onChange({ ...values, [key]: value });
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--tf-space-4)" }}>
      {sorted.map((group) => {
        const fields = fieldsByGroup.get(group.id) ?? [];
        if (fields.length === 0) return null;
        return (
          <FieldGroupSection key={group.id} group={group}>
            {fields.map((key) => (
              <SchemaField
                key={key}
                name={key}
                schema={properties[key]}
                value={values[key]}
                required={required.has(key)}
                onChange={(v) => setField(key, v)}
              />
            ))}
          </FieldGroupSection>
        );
      })}
      {ungrouped.length > 0 && (
        <div style={{ display: "flex", flexDirection: "column", gap: "var(--tf-space-3)" }}>
          {ungrouped.map((key) => (
            <SchemaField
              key={key}
              name={key}
              schema={properties[key]}
              value={values[key]}
              required={required.has(key)}
              onChange={(v) => setField(key, v)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function FieldGroupSection({ group, children }: { group: FieldGroup; children: React.ReactNode }) {
  return (
    <fieldset
      style={{
        border: "1px solid var(--tf-color-border)",
        borderRadius: "var(--tf-radius-m)",
        padding: "var(--tf-space-3)",
        margin: 0,
      }}
    >
      <legend
        style={{
          fontWeight: 600,
          fontSize: "var(--tf-text-small)",
          color: "var(--tf-color-text-secondary)",
          padding: "0 var(--tf-space-2)",
        }}
      >
        {group.label}
      </legend>
      <div style={{ display: "flex", flexDirection: "column", gap: "var(--tf-space-3)" }}>
        {children}
      </div>
    </fieldset>
  );
}

interface FieldProps {
  name: string;
  schema: JSONSchema;
  value: unknown;
  required: boolean;
  onChange: (value: unknown) => void;
}

function SchemaField({ name, schema, value, required, onChange }: FieldProps) {
  const ui: TFUIMetadata = schema["x-tf-ui"] ?? {};
  const widget = ui.widget ?? resolveWidget(schema);
  const label = schema.title ?? name;
  const desc = schema.description;

  const labelEl = (
    <label
      style={{
        display: "block",
        fontWeight: 500,
        fontSize: "var(--tf-text-small)",
        marginBottom: "var(--tf-space-1)",
        color: "var(--tf-color-text)",
      }}
    >
      {label}
      {required && <span style={{ color: "var(--tf-color-bad)", marginLeft: 4 }}>*</span>}
    </label>
  );

  const descEl = desc ? (
    <div
      style={{
        fontSize: "var(--tf-text-xs)",
        color: "var(--tf-color-text-muted)",
        marginBottom: "var(--tf-space-1)",
      }}
    >
      {desc}
    </div>
  ) : null;

  const inputStyle: React.CSSProperties = {
    width: "100%",
    padding: "var(--tf-space-2)",
    fontSize: "var(--tf-text-small)",
    border: "1px solid var(--tf-color-border)",
    borderRadius: "var(--tf-radius-s)",
    background: "var(--tf-color-surface)",
    color: "var(--tf-color-text)",
    boxSizing: "border-box",
  };

  if (widget === "locked") {
    return (
      <div>
        {labelEl}
        {descEl}
        <LockedField
          value={value ?? schema.default}
          reason={ui.locked_reason}
        />
      </div>
    );
  }

  if (widget === "select" && schema.enum) {
    return (
      <div>
        {labelEl}
        {descEl}
        <select
          value={String(value ?? schema.default ?? "")}
          onChange={(e) => onChange(e.target.value)}
          style={inputStyle}
        >
          {schema.enum.map((opt) => (
            <option key={String(opt)} value={String(opt)}>
              {String(opt) || "(none)"}
            </option>
          ))}
        </select>
      </div>
    );
  }

  if (widget === "checkbox") {
    return (
      <div>
        <label
          style={{
            display: "flex",
            alignItems: "center",
            gap: "var(--tf-space-2)",
            fontSize: "var(--tf-text-small)",
            cursor: "pointer",
          }}
        >
          <input
            type="checkbox"
            checked={Boolean(value ?? schema.default ?? false)}
            onChange={(e) => onChange(e.target.checked)}
          />
          {label}
          {required && <span style={{ color: "var(--tf-color-bad)" }}>*</span>}
        </label>
        {descEl}
      </div>
    );
  }

  if (widget === "number") {
    return (
      <div>
        {labelEl}
        {descEl}
        <input
          type="number"
          value={value != null ? Number(value) : (schema.default as number) ?? ""}
          min={schema.minimum}
          max={schema.maximum}
          onChange={(e) => onChange(e.target.value === "" ? undefined : Number(e.target.value))}
          style={inputStyle}
        />
      </div>
    );
  }

  if (widget === "key-value-map") {
    return (
      <div>
        {labelEl}
        {descEl}
        <KeyValueEditor
          value={(value ?? schema.default ?? {}) as Record<string, string>}
          onChange={onChange}
        />
      </div>
    );
  }

  if (widget === "list-of-strings") {
    return (
      <div>
        {labelEl}
        {descEl}
        <ListEditor
          value={(value ?? schema.default ?? []) as string[]}
          onChange={onChange}
        />
      </div>
    );
  }

  if (widget === "nullable-object") {
    const enabled = value != null;
    const objValue = (value ?? {}) as Record<string, unknown>;
    return (
      <div>
        <label
          style={{
            display: "flex",
            alignItems: "center",
            gap: "var(--tf-space-2)",
            fontWeight: 500,
            fontSize: "var(--tf-text-small)",
            marginBottom: "var(--tf-space-2)",
            cursor: "pointer",
          }}
        >
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => onChange(e.target.checked ? {} : null)}
          />
          {label}
        </label>
        {descEl}
        {enabled && schema.properties && (
          <div
            style={{
              paddingLeft: "var(--tf-space-4)",
              borderLeft: "2px solid var(--tf-color-border)",
              display: "flex",
              flexDirection: "column",
              gap: "var(--tf-space-3)",
            }}
          >
            {Object.entries(schema.properties).map(([k, sub]) => (
              <SchemaField
                key={k}
                name={k}
                schema={sub}
                value={objValue[k]}
                required={(schema.required ?? []).includes(k)}
                onChange={(v) => onChange({ ...objValue, [k]: v })}
              />
            ))}
          </div>
        )}
      </div>
    );
  }

  if (widget === "pick-existing") {
    return (
      <div>
        {labelEl}
        {descEl}
        <input
          type="text"
          value={String(value ?? schema.default ?? "")}
          onChange={(e) => onChange(e.target.value)}
          placeholder={`Select ${label}...`}
          style={inputStyle}
        />
        <div style={{ fontSize: "var(--tf-text-xs)", color: "var(--tf-color-text-muted)", marginTop: 2 }}>
          Pick from existing {ui.pick?.module_type ?? "resources"}
        </div>
      </div>
    );
  }

  // Default: text input
  return (
    <div>
      {labelEl}
      {descEl}
      <input
        type="text"
        value={String(value ?? schema.default ?? "")}
        onChange={(e) => onChange(e.target.value)}
        style={inputStyle}
      />
    </div>
  );
}

function resolveWidget(schema: JSONSchema): string {
  if (schema.enum) return "select";
  const type = Array.isArray(schema.type) ? schema.type[0] : schema.type;
  if (type === "boolean") return "checkbox";
  if (type === "integer" || type === "number") return "number";
  if (type === "array") return "list-of-strings";
  if (type === "object" && schema.additionalProperties) return "key-value-map";
  return "text";
}
