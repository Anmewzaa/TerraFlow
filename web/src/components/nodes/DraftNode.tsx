import { Handle, Position, type NodeProps } from "@xyflow/react";
import type { CanvasNode } from "../../types/changeset";

interface DraftNodeData {
  canvasNode: CanvasNode;
  onEdit: (id: string) => void;
  onRemove: (id: string) => void;
  [key: string]: unknown;
}

export function DraftNode({ data }: NodeProps) {
  const { canvasNode, onEdit, onRemove } = data as DraftNodeData;

  return (
    <div
      style={{
        padding: "var(--tf-space-2) var(--tf-space-3)",
        background: "var(--tf-color-surface)",
        border: "2px dashed var(--tf-color-accent)",
        borderRadius: "var(--tf-radius-m)",
        minWidth: 180,
        fontSize: "var(--tf-text-small)",
      }}
    >
      <Handle type="target" position={Position.Left} />
      <div style={{ display: "flex", alignItems: "center", gap: "var(--tf-space-2)" }}>
        <span
          style={{
            fontSize: "10px",
            padding: "1px 6px",
            borderRadius: "var(--tf-radius-s)",
            background: "var(--tf-color-accent)",
            color: "#fff",
            fontWeight: 600,
            textTransform: "uppercase",
            letterSpacing: "0.5px",
          }}
        >
          draft
        </span>
        <span
          style={{
            fontFamily: "var(--tf-font-mono)",
            fontWeight: 500,
            color: "var(--tf-color-text)",
          }}
        >
          {canvasNode.instance_name}
        </span>
      </div>
      <div
        style={{
          fontSize: "11px",
          color: "var(--tf-color-text-muted)",
          marginTop: "var(--tf-space-1)",
        }}
      >
        {canvasNode.module_name} / {canvasNode.module_version}
      </div>
      <div
        style={{
          fontSize: "11px",
          color: "var(--tf-color-text-muted)",
          fontFamily: "var(--tf-font-mono)",
        }}
      >
        → {canvasNode.target_folder}/
      </div>
      <div style={{ display: "flex", gap: "var(--tf-space-2)", marginTop: "var(--tf-space-2)" }}>
        <button
          onClick={(e) => { e.stopPropagation(); onEdit(canvasNode.id); }}
          style={{
            padding: "2px 8px",
            fontSize: "11px",
            cursor: "pointer",
            borderRadius: "var(--tf-radius-s)",
            border: "1px solid var(--tf-color-border)",
            background: "transparent",
            color: "var(--tf-color-text-secondary)",
          }}
        >
          Edit
        </button>
        <button
          onClick={(e) => { e.stopPropagation(); onRemove(canvasNode.id); }}
          style={{
            padding: "2px 8px",
            fontSize: "11px",
            cursor: "pointer",
            borderRadius: "var(--tf-radius-s)",
            border: "1px solid var(--tf-color-border)",
            background: "transparent",
            color: "var(--tf-color-bad)",
          }}
        >
          Remove
        </button>
      </div>
      <Handle type="source" position={Position.Right} />
    </div>
  );
}
