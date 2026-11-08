import type { NodeProps } from "@xyflow/react";
import type { CanvasNode } from "../../types/changeset";

const CONTAINER_TYPES = new Set(["resource-group", "resource_group", "virtual-network", "virtual_network"]);

export function isDraftContainer(moduleName: string): boolean {
  return CONTAINER_TYPES.has(moduleName);
}

interface DraftContainerData {
  canvasNode: CanvasNode;
  onEdit: (id: string) => void;
  onRemove: (id: string) => void;
  [key: string]: unknown;
}

export function DraftContainerNode({ data }: NodeProps) {
  const { canvasNode, onEdit, onRemove } = data as DraftContainerData;
  const displayName = (canvasNode.values?.name as string) || canvasNode.instance_name;

  return (
    <div className="tf-container-node tf-container-node--draft">
      <div className="tf-container-node__label">
        <span className="tf-dot" style={{ background: "var(--tf-color-accent)" }} />
        {displayName}
        <span
          style={{
            marginLeft: "auto",
            fontSize: 9,
            fontWeight: 600,
            padding: "1px 6px",
            borderRadius: "var(--tf-radius-pill)",
            background: "rgba(198,138,28,0.12)",
            color: "var(--tf-color-accent)",
          }}
        >
          draft
        </span>
      </div>
      <div style={{ position: "absolute", top: 8, right: 12, display: "flex", gap: "var(--tf-space-1)" }}>
        <button
          onClick={(e) => { e.stopPropagation(); onEdit(canvasNode.id); }}
          style={{
            padding: "1px 6px", fontSize: 10, cursor: "pointer",
            borderRadius: "var(--tf-radius-sm)",
            border: "1px solid var(--tf-color-border)",
            background: "var(--tf-color-surface)",
            color: "var(--tf-color-text-secondary)",
            fontFamily: "inherit",
          }}
        >
          Edit
        </button>
        <button
          onClick={(e) => { e.stopPropagation(); onRemove(canvasNode.id); }}
          style={{
            padding: "1px 6px", fontSize: 10, cursor: "pointer",
            borderRadius: "var(--tf-radius-sm)",
            border: "1px solid var(--tf-color-border)",
            background: "var(--tf-color-surface)",
            color: "var(--tf-color-error)",
            fontFamily: "inherit",
          }}
        >
          Remove
        </button>
      </div>
    </div>
  );
}
