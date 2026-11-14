import { useMemo } from "react";
import { valueText } from "../../../lib/valueText";
import { currentLink, linkExpr, linkTargets, linkValue } from "../../../lib/links";
import { useGraphContext } from "../../../contexts/GraphContext";
import type { TFUIMetadata } from "../../../types/catalog";
import type { ProjectGraph } from "../../../types/graph";
import type { CanvasNode } from "../../../types/changeset";

interface Props {
  label: string;
  value: unknown;
  onChange: (value: unknown, siblings?: Record<string, unknown>) => void;
  pick: NonNullable<TFUIMetadata["pick"]>;
  graph: ProjectGraph | null;
  draftNodes?: CanvasNode[];
  disabled?: boolean;
}

export function PickExistingField(props: Props) {
  // A field that asks for a link (8.11) lists what offers it; one that names a module type lists those.
  return props.pick.link ? <LinkField {...props} /> : <TypeField {...props} />;
}

/** A link: the target is chosen among the resources that advertise the kind, and the reference is written. */
function LinkField({ label, value, onChange, pick, graph, draftNodes, disabled }: Props) {
  const { modules } = useGraphContext();
  const targets = useMemo(() => linkTargets(pick.link ?? "", modules ?? [], graph, draftNodes), [pick.link, modules, graph, draftNodes]);
  const current = currentLink(value);
  const known = targets.some((t) => linkExpr(t) === current);
  return (
    <div style={{ marginBottom: "var(--tf-space-3)" }}>
      <label style={{ display: "block", fontSize: "var(--tf-text-small)", fontWeight: 500, marginBottom: "var(--tf-space-1)", color: "var(--tf-color-text-secondary)" }}>
        {label}
      </label>
      <select
        aria-label={label}
        value={current}
        disabled={disabled || (targets.length === 0 && !current)}
        className="tf-input"
        style={{ fontFamily: "var(--tf-font-mono)" }}
        onChange={(e) => {
          const t = targets.find((x) => linkExpr(x) === e.target.value);
          if (!t) return onChange(pick.list ? [] : undefined);
          const { value: v, siblings } = linkValue(t, pick.list, pick.fills);
          onChange(v, siblings);
        }}
      >
        <option value="">{targets.length === 0 ? "Nothing on the canvas offers this yet" : "Choose a resource"}</option>
        {current && !known && <option value={current}>{current} (not on the canvas)</option>}
        {targets.map((t) => (
          <option key={`${t.folder}/${t.label}`} value={linkExpr(t)}>
            {t.name} ({t.folder}/){t.draft ? " [draft]" : ""}
          </option>
        ))}
      </select>
      {targets.length === 0 && !current && (
        <div style={{ marginTop: 4, fontSize: "var(--tf-text-caption)", color: "var(--tf-color-text-muted)" }}>
          Add a resource that can be linked here to the canvas first, then choose it.
        </div>
      )}
    </div>
  );
}

function TypeField({ label, value, onChange, pick, graph, draftNodes, disabled }: Props) {
  const options = useMemo(() => {
    const matches: { id: string; name: string; folder: string; draft?: boolean }[] = [];

    if (graph) {
      for (const folder of graph.folders) {
        for (const node of folder.nodes) {
          if (node.kind === "module" && node.type === pick.module_type && node.name) {
            matches.push({ id: node.id, name: node.name, folder: folder.name });
          }
        }
      }
    }

    if (draftNodes) {
      for (const dn of draftNodes) {
        if (dn.module_name === pick.module_type && valueText(dn.values?.name)) {
          matches.push({
            id: dn.id,
            name: valueText(dn.values.name),
            folder: dn.target_folder,
            draft: true,
          });
        }
      }
    }

    return matches;
  }, [graph, draftNodes, pick.module_type]);

  return (
    <div style={{ marginBottom: "var(--tf-space-3)" }}>
      <label
        style={{
          display: "block",
          fontSize: "var(--tf-text-small)",
          fontWeight: 500,
          marginBottom: "var(--tf-space-1)",
          color: "var(--tf-color-text-secondary)",
        }}
      >
        {label}
      </label>
      <select
        value={typeof value === "string" ? value : ""}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled || options.length === 0}
        className="tf-input"
        style={{ fontFamily: "var(--tf-font-mono)" }}
      >
        <option value="">
          {options.length === 0 ? "No matching resource yet" : "Select existing resource"}
        </option>
        {options.map((opt) => (
          <option key={opt.id} value={opt.name}>
            {opt.name} ({opt.folder}/) {opt.draft ? "[draft]" : ""}
          </option>
        ))}
      </select>
      {options.length === 0 && (
        <div style={{ marginTop: 4, fontSize: "var(--tf-text-caption)", color: "var(--tf-color-text-muted)" }}>
          Add a {pick.module_type} to the canvas (or the repo) first, then pick it here.
        </div>
      )}
    </div>
  );
}
