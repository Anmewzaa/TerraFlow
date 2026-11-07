import type { NodeProps } from "@xyflow/react";
import type { GraphNode } from "../../types/graph";

export function FolderGroupNode({ data }: NodeProps) {
  const label = data.label as string;
  const node = data.node as GraphNode | undefined;
  const isRG = node?.type === "resource-group";
  const isVNet = node?.type === "virtual-network";

  const typeLabel = isRG ? "RESOURCE GROUP" : isVNet ? "VIRTUAL NETWORK" : undefined;

  return (
    <div
      style={{
        width: "100%",
        height: "100%",
        background: "var(--tf-color-canvas)",
        border: `1px dashed var(--tf-color-border)`,
        borderRadius: "var(--tf-radius-panel)",
        position: "relative",
      }}
    >
      <div style={{ position: "absolute", top: 8, left: 12 }}>
        {typeLabel && (
          <div
            style={{
              fontSize: 9,
              fontWeight: 600,
              color: "var(--tf-color-text-muted)",
              textTransform: "uppercase",
              letterSpacing: "0.05em",
              marginBottom: 2,
            }}
          >
            {typeLabel}
          </div>
        )}
        <div
          style={{
            fontSize: 11,
            fontWeight: 600,
            fontFamily: "var(--tf-font-mono)",
            color: "var(--tf-color-text-secondary)",
          }}
        >
          {label}
        </div>
      </div>
    </div>
  );
}
