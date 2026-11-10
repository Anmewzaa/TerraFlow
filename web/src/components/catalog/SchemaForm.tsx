import type { JSONSchema, FieldGroup, TFUIMetadata } from "../../types/catalog";
import { isExpr } from "../../lib/valueText";
import { KeyValueEditor } from "./widgets/KeyValueEditor";
import { ListEditor } from "./widgets/ListEditor";
import { LockedField } from "./widgets/LockedField";
import { PickExistingField } from "./widgets/PickExistingField";
import { useGraphContext } from "../../contexts/GraphContext";
import { useGenerationSettings } from "../../contexts/GenerationContext";
import { suggestToken } from "../../lib/generation";

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
    <fieldset className="tf-form-group">
      <legend>{group.label}</legend>
      <div style={{ display: "flex", flexDirection: "column", gap: "var(--tf-space-4)" }}>
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
  const generation = useGenerationSettings();
  const ui: TFUIMetadata = schema["x-tf-ui"] ?? {};
  const widget = ui.widget ?? resolveWidget(schema);
  const label = schema.title ?? name;
  const desc = schema.description;
  const { graph, draftNodes } = useGraphContext();

  const labelEl = (
    <label
      style={{
        display: "block",
        fontWeight: 600,
        fontSize: "var(--tf-text-small)",
        marginBottom: 2,
        color: "var(--tf-color-text)",
      }}
    >
      {label}
      {required && <span style={{ color: "var(--tf-color-bad)", marginLeft: 4 }}>*</span>}
    </label>
  );

  // A reference read from the repository (var.x, module.a.id, ...): show it as
  // written and keep it, instead of a text box that would turn it into "[object Object]".
  if (isExpr(value)) {
    return (
      <div>
        {labelEl}
        <div
          className="tf-input"
          style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8 }}
          title="Kept as written in the repository"
        >
          <code style={{ fontSize: 12, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{value.$expr}</code>
          <button
            type="button"
            className="tf-btn tf-btn--ghost"
            style={{ height: 24, padding: "0 8px", fontSize: 11, flexShrink: 0 }}
            onClick={() => onChange(undefined)}
          >
            Use a value
          </button>
        </div>
        {desc && <small style={{ color: "var(--tf-color-text-muted)" }}>{desc}</small>}
      </div>
    );
  }

  const descEl = desc ? (
    <div
      style={{
        fontSize: "var(--tf-text-caption)",
        lineHeight: 1.4,
        color: "var(--tf-color-text-muted)",
        marginBottom: "var(--tf-space-2)",
      }}
    >
      {desc}
    </div>
  ) : null;

  const inputStyle: React.CSSProperties = {};

  if (widget === "from-parent") {
    return (
      <div>
        {labelEl}
        {descEl}
        <div
          style={{
            padding: "var(--tf-space-2) var(--tf-space-3)",
            borderRadius: "var(--tf-radius-control)",
            border: "1px dashed var(--tf-color-border-strong)",
            background: "var(--tf-color-surface-subtle)",
            color: "var(--tf-color-text-muted)",
            fontSize: "var(--tf-text-small)",
          }}
        >
          {value ? "Filled in from the parent when HCL is generated" : "Not placed in a parent yet: drag this resource into one"}
        </div>
      </div>
    );
  }

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
          className="tf-input" style={inputStyle}
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
          className="tf-input" style={inputStyle}
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

  if (widget === "pick-existing" && ui.pick) {
    return (
      <div>
        {descEl}
        <PickExistingField
          label={label + (required ? " *" : "")}
          value={value ?? schema.default}
          onChange={(v) => onChange(v)}
          pick={ui.pick}
          graph={graph}
          draftNodes={draftNodes}
        />
      </div>
    );
  }

  // Default: text input
  const text = String(value ?? schema.default ?? "");
  const vars = generation.variables ?? [];
  const suggestion = suggestToken(text, generation);
  return (
    <div className="group">
      {labelEl}
      {descEl}
      <input
        type="text"
        value={text}
        onChange={(e) => onChange(e.target.value)}
        className="tf-input" style={inputStyle}
      />
      {suggestion && (
        <button
          type="button"
          className="mt-1 text-left text-[11px] underline"
          style={{ color: "var(--tf-color-action)" }}
          onClick={() => onChange(suggestion.replaced)}
        >
          Use {"{" + suggestion.name + "}"} instead of “{suggestion.value}”
        </button>
      )}
      {vars.length > 0 && (
        <div className="mt-1 hidden flex-wrap items-center gap-1 text-[11px] group-focus-within:flex" style={{ color: "var(--tf-color-text-muted)" }}>
          Project variables:
          {vars.map((v) => (
            <button
              key={v.name}
              type="button"
              className="rounded border px-1 font-mono"
              style={{ borderColor: "var(--tf-color-border)" }}
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => onChange(text + "{" + v.name + "}")}
              title={v.description || v.name}
            >
              {"{" + v.name + "}"}
            </button>
          ))}
        </div>
      )}
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
