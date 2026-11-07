import { Handle, Position, type NodeProps } from "@xyflow/react";
import type { GraphExternal } from "../../types/graph";

export function ExternalNode({ data }: NodeProps) {
  const ext = data.external as GraphExternal;
  return (
    <div
      style={{
        background: "var(--tf-color-surface)",
        border: "1px dashed var(--tf-color-text-muted)",
        borderRadius: "var(--tf-radius-m)",
        padding: "var(--tf-space-2) var(--tf-space-3)",
        minWidth: 200,
        fontSize: "var(--tf-text-small)",
        opacity: 0.85,
      }}
    >
      <Handle type="target" position={Position.Top} style={{ visibility: "hidden" }} />
      <div style={{ display: "flex", alignItems: "center", gap: "var(--tf-space-2)" }}>
        <span className="tf-badge" data-tone="info" style={{ fontSize: 10 }}>
          external
        </span>
        <span style={{ color: "var(--tf-color-text-muted)", fontSize: 11 }}>
          {ext.kind}
        </span>
      </div>
      <div style={{ marginTop: "var(--tf-space-1)" }}>
        <code className="tf-ident" style={{ fontSize: 12 }}>{ext.name}</code>
      </div>
      <div style={{ color: "var(--tf-color-text-muted)", fontSize: 10, marginTop: 2 }}>
        source: {ext.source}
      </div>
      <Handle type="source" position={Position.Bottom} style={{ visibility: "hidden" }} />
    </div>
  );
}
