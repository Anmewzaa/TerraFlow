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

  const isRG = canvasNode.module_name === "resource_group" || canvasNode.module_name === "resource-group";
  const typeLabel = isRG ? "RESOURCE GROUP" : "VIRTUAL NETWORK";

  const displayName =
    (canvasNode.values?.name as string) || canvasNode.instance_name;

  return (
    <div
      style={{
        width: "100%",
        height: "100%",
        background: "var(--tf-color-canvas)",
        border: "2px dashed var(--tf-color-accent)",
        borderRadius: "var(--tf-radius-panel)",
        position: "relative",
      }}
    >
      <div style={{ position: "absolute", top: 8, left: 12, display: "flex", alignItems: "flex-start", gap: "var(--tf-space-2)" }}>
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: "var(--tf-space-2)" }}>
            <span
              style={{
                fontSize: 9,
                padding: "1px 5px",
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
                fontSize: 9,
                fontWeight: 600,
                color: "var(--tf-color-text-muted)",
                textTransform: "uppercase",
                letterSpacing: "0.05em",
              }}
            >
              {typeLabel}
            </span>
          </div>
          <div
            style={{
              fontSize: 11,
              fontWeight: 600,
              fontFamily: "var(--tf-font-mono)",
              color: "var(--tf-color-text-secondary)",
              marginTop: 2,
            }}
          >
            {displayName}
          </div>
          <div style={{ fontSize: 10, color: "var(--tf-color-text-muted)" }}>
            {canvasNode.module_version} · {canvasNode.target_folder}/
          </div>
        </div>
      </div>
      <div style={{ position: "absolute", top: 8, right: 12, display: "flex", gap: "var(--tf-space-1)" }}>
        <button
          onClick={(e) => { e.stopPropagation(); onEdit(canvasNode.id); }}
          style={{
            padding: "1px 6px",
            fontSize: 10,
            cursor: "pointer",
            borderRadius: "var(--tf-radius-s)",
            border: "1px solid var(--tf-color-border)",
            background: "var(--tf-color-surface)",
            color: "var(--tf-color-text-secondary)",
          }}
        >
          Edit
        </button>
        <button
          onClick={(e) => { e.stopPropagation(); onRemove(canvasNode.id); }}
          style={{
            padding: "1px 6px",
            fontSize: 10,
            cursor: "pointer",
            borderRadius: "var(--tf-radius-s)",
            border: "1px solid var(--tf-color-border)",
            background: "var(--tf-color-surface)",
            color: "var(--tf-color-bad)",
          }}
        >
          Remove
        </button>
      </div>
    </div>
  );
}
