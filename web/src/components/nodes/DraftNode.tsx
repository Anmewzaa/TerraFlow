import { Handle, Position, type NodeProps } from "@xyflow/react";
import type { CanvasNode } from "../../types/changeset";
import { brandIdentity } from "../../design-system";

const domainColors = brandIdentity.domains;

const moduleDomainMap: Record<string, keyof typeof domainColors> = {
  "resource-group": "neutral",
  "resource_group": "neutral",
  "virtual-network": "network",
  "virtual_network": "network",
  "network-security-group": "network",
  "network_security_group": "network",
  "key-vault": "security",
  "key_vault": "security",
  "storage-account": "data",
  "storage_account": "data",
};

interface DraftNodeData {
  canvasNode: CanvasNode;
  onEdit: (id: string) => void;
  onRemove: (id: string) => void;
  [key: string]: unknown;
}

export function DraftNode({ data }: NodeProps) {
  const { canvasNode, onEdit, onRemove } = data as DraftNodeData;

  const domain = moduleDomainMap[canvasNode.module_name] ?? "neutral";
  const color = domainColors[domain].color;
  const displayName =
    (canvasNode.values?.name as string) || canvasNode.instance_name;

  return (
    <div
      style={{
        background: "var(--tf-color-surface)",
        border: "2px dashed var(--tf-color-border)",
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
          style={{
            fontSize: 10,
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
        <span style={{ color, fontWeight: 600, fontSize: 11 }}>
          {canvasNode.module_name}
        </span>
      </div>
      <div style={{ marginTop: "var(--tf-space-1)" }}>
        <code
          className="tf-ident"
          style={{ fontSize: 12 }}
        >
          {displayName}
        </code>
      </div>
      <div style={{ color: "var(--tf-color-text-muted)", fontSize: 10, marginTop: 2 }}>
        {canvasNode.module_version} · {canvasNode.target_folder}/
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
      <Handle type="source" position={Position.Bottom} style={{ visibility: "hidden" }} />
    </div>
  );
}
