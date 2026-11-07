import type { GraphNode, GraphExternal } from "../types/graph";
import { brandIdentity } from "../design-system";

function isGraphNode(n: GraphNode | GraphExternal): n is GraphNode {
  return "kind" in n && "folder" in n;
}

interface NodeDetailProps {
  node: GraphNode | GraphExternal;
  onClose: () => void;
}

export function NodeDetail({ node, onClose }: NodeDetailProps) {
  return (
    <aside
      style={{
        width: 340,
        borderLeft: "1px solid var(--tf-color-border)",
        background: "var(--tf-color-surface)",
        padding: "var(--tf-space-4)",
        overflowY: "auto",
        fontSize: "var(--tf-text-small)",
      }}
    >
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <h3 style={{ margin: 0, fontSize: "var(--tf-text-body)" }}>
          {isGraphNode(node) ? node.id : node.name}
        </h3>
        <button
          onClick={onClose}
          style={{
            background: "none",
            border: "none",
            cursor: "pointer",
            fontSize: 18,
            color: "var(--tf-color-text-muted)",
          }}
        >
          ×
        </button>
      </div>

      {isGraphNode(node) ? <ModuleDetail node={node} /> : <ExternalDetail ext={node} />}
    </aside>
  );
}

function ModuleDetail({ node }: { node: GraphNode }) {
  const color = brandIdentity.domains[node.domain]?.color ?? brandIdentity.domains.neutral.color;

  return (
    <div style={{ marginTop: "var(--tf-space-3)" }}>
      <div style={{ display: "flex", gap: "var(--tf-space-2)", marginBottom: "var(--tf-space-2)" }}>
        <span className="tf-badge" data-tone={node.kind === "data" ? "info" : "neutral"}>
          {node.kind}
        </span>
        <span style={{ color, fontWeight: 600 }}>{node.type}</span>
      </div>

      <dl style={{ margin: 0 }}>
        <dt style={{ color: "var(--tf-color-text-muted)", marginTop: "var(--tf-space-2)" }}>Name</dt>
        <dd style={{ margin: 0 }}>
          {node.nameState === "resolved" ? (
            <code className="tf-ident">{node.name}</code>
          ) : (
            <span className="tf-unknown">unknown</span>
          )}
        </dd>

        <dt style={{ color: "var(--tf-color-text-muted)", marginTop: "var(--tf-space-2)" }}>Folder</dt>
        <dd style={{ margin: 0 }}>{node.folder}</dd>

        {node.source && (
          <>
            <dt style={{ color: "var(--tf-color-text-muted)", marginTop: "var(--tf-space-2)" }}>Source</dt>
            <dd style={{ margin: 0, wordBreak: "break-all" }}>
              <code className="tf-ident" style={{ fontSize: 11 }}>{node.source}</code>
            </dd>
          </>
        )}

        <dt style={{ color: "var(--tf-color-text-muted)", marginTop: "var(--tf-space-3)" }}>Inputs</dt>
        <dd style={{ margin: 0 }}>
          <InputsTable inputs={node.inputs} />
        </dd>
      </dl>
    </div>
  );
}

function ExternalDetail({ ext }: { ext: GraphExternal }) {
  return (
    <div style={{ marginTop: "var(--tf-space-3)" }}>
      <span className="tf-badge" data-tone="info">external</span>
      <dl style={{ margin: 0, marginTop: "var(--tf-space-2)" }}>
        <dt style={{ color: "var(--tf-color-text-muted)" }}>Kind</dt>
        <dd style={{ margin: 0 }}>{ext.kind}</dd>
        <dt style={{ color: "var(--tf-color-text-muted)", marginTop: "var(--tf-space-2)" }}>Name</dt>
        <dd style={{ margin: 0 }}><code className="tf-ident">{ext.name}</code></dd>
        <dt style={{ color: "var(--tf-color-text-muted)", marginTop: "var(--tf-space-2)" }}>Source</dt>
        <dd style={{ margin: 0 }}>{ext.source}</dd>
      </dl>
    </div>
  );
}

function InputsTable({ inputs }: { inputs: Record<string, unknown> }) {
  const entries = Object.entries(inputs);
  if (entries.length === 0) {
    return <span className="tf-subtle">none</span>;
  }
  return (
    <table style={{ width: "100%", fontSize: 11, borderCollapse: "collapse", marginTop: "var(--tf-space-1)" }}>
      <tbody>
        {entries.map(([key, val]) => (
          <tr key={key} style={{ borderBottom: "1px solid var(--tf-color-border)" }}>
            <td style={{ padding: "2px 4px", color: "var(--tf-color-text-secondary)", whiteSpace: "nowrap", verticalAlign: "top" }}>
              <code className="tf-ident">{key}</code>
            </td>
            <td style={{ padding: "2px 4px", wordBreak: "break-all" }}>
              <InputValue value={val} />
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function InputValue({ value }: { value: unknown }) {
  if (value && typeof value === "object" && "$unknown" in (value as Record<string, unknown>)) {
    return <span className="tf-unknown">unknown</span>;
  }
  if (typeof value === "string") {
    return <code className="tf-ident">{value}</code>;
  }
  if (typeof value === "boolean") {
    return <code className="tf-ident">{String(value)}</code>;
  }
  if (typeof value === "number") {
    return <code className="tf-ident">{String(value)}</code>;
  }
  if (Array.isArray(value)) {
    return <code className="tf-ident" style={{ fontSize: 10 }}>{JSON.stringify(value)}</code>;
  }
  if (value && typeof value === "object") {
    return (
      <details>
        <summary style={{ cursor: "pointer", color: "var(--tf-color-text-muted)" }}>
          {"{…}"}
        </summary>
        <pre style={{ fontSize: 10, margin: 0, whiteSpace: "pre-wrap" }}>
          {JSON.stringify(value, null, 2)}
        </pre>
      </details>
    );
  }
  return <span className="tf-unknown">unknown</span>;
}
