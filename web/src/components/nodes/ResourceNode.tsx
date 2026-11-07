import { Handle, Position, type NodeProps } from "@xyflow/react";
import type { GraphNode } from "../../types/graph";
import { brandIdentity } from "../../design-system";

const domainColors = brandIdentity.domains;

export function ResourceNode({ data }: NodeProps) {
  const node = data.node as GraphNode;
  const color = domainColors[node.domain]?.color ?? domainColors.neutral.color;

  const version = node.source ? extractVersion(node.source) : null;
  const catalogKey = node.type;

  return (
    <div
      style={{
        background: "var(--tf-color-surface)",
        border: "1px solid var(--tf-color-border)",
        borderRadius: "var(--tf-radius-m)",
        borderLeft: `4px solid ${color}`,
        padding: "var(--tf-space-2) var(--tf-space-3)",
        minWidth: 220,
        fontSize: "var(--tf-text-small)",
      }}
    >
      <Handle type="target" position={Position.Top} style={{ visibility: "hidden" }} />
      <div style={{ display: "flex", alignItems: "center", gap: "var(--tf-space-2)" }}>
        <span
          className="tf-badge"
          data-tone={node.kind === "data" ? "info" : "neutral"}
          style={{ fontSize: 10 }}
        >
          {node.kind}
        </span>
        <span style={{ color, fontWeight: 600, fontSize: 11 }}>
          {catalogKey}
        </span>
      </div>
      <div style={{ marginTop: "var(--tf-space-1)" }}>
        {node.nameState === "resolved" ? (
          <code className="tf-ident" style={{ fontSize: 12 }}>{node.name}</code>
        ) : (
          <span className="tf-unknown" style={{ fontSize: 12 }}>unknown</span>
        )}
      </div>
      {version && (
        <div style={{ color: "var(--tf-color-text-muted)", fontSize: 10, marginTop: 2 }}>
          {version}
        </div>
      )}
      <Handle type="source" position={Position.Bottom} style={{ visibility: "hidden" }} />
    </div>
  );
}

function extractVersion(source: string): string | null {
  const m = source.match(/\?ref=([^&]+)/);
  return m ? m[1] : null;
}
