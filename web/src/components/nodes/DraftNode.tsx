import { Handle, Position, type NodeProps } from "@xyflow/react";
import { displayName as nodeDisplayName } from "../../lib/valueText";
import { Pencil, Trash2 } from "lucide-react";
import type { CanvasNode } from "../../types/changeset";
import type { Issue } from "../../lib/validate";
import { domainCssVars, domainBgOpacity, getAzureIcon, moduleDomainMap } from "./domainStyles";

interface DraftNodeData {
  canvasNode: CanvasNode;
  title: string;
  domain?: string;
  issues?: Issue[];
  /** The project's policy warnings for this resource (8.13). */
  warnings?: Issue[];
  /** Picked for a stack template (8.15). */
  picked?: boolean;
  onEdit: (id: string) => void;
  onRemove: (id: string) => void;
  [key: string]: unknown;
}

export function DraftNode({ data, selected }: NodeProps) {
  const { canvasNode, title, domain: hinted, onEdit, onRemove, issues = [], warnings = [], picked = false } = data as DraftNodeData;
  const invalid = issues.length > 0;
  const domain = hinted ?? moduleDomainMap[canvasNode.module_name] ?? "neutral";
  const color = domainCssVars[domain] ?? domainCssVars.neutral;
  const iconBg = domainBgOpacity[domain] ?? domainBgOpacity.neutral;
  const iconSrc = getAzureIcon(canvasNode.module_name);
  const displayName = nodeDisplayName(canvasNode.values, canvasNode.instance_name);

  return (
    <div
      className={`tf-node tf-node--draft${selected ? " tf-node--selected" : ""}${invalid ? " tf-node--invalid" : ""}${picked ? " tf-node--picked" : ""}`}
      style={{ width: "100%", height: "100%" }}
      title={invalid ? issues.map((i) => i.text).join("\n") : undefined}
    >
      <Handle type="target" position={Position.Top} style={{ visibility: "hidden" }} />
      <div className="tf-node__accent" style={{ background: color }} />
      <div className="tf-node__header">
        <div className="tf-node__icon" style={{ background: iconBg, color }}>
          {iconSrc ? (
            <img src={iconSrc} alt="" />
          ) : (
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <rect x="3" y="3" width="18" height="18" rx="2" />
            </svg>
          )}
        </div>
        <div style={{ minWidth: 0, flex: 1 }}>
          <div className="tf-node__title">{title}</div>
          <div className="tf-node__type">{canvasNode.kind === "resource" ? "raw resource" : canvasNode.module_version}</div>
        </div>
        {invalid ? <span className="tf-invalid-badge">{issues.length} to fix</span> : warnings.length > 0 ? <span className="tf-warn-badge">{warnings.length} to review</span> : <span className="tf-draft-badge">draft</span>}
      </div>
      <div className="tf-node__body">
        <span className="tf-node__ident">{displayName}</span>
        <div className="tf-node__meta">
          <span>{canvasNode.target_folder}/</span>
        </div>
      </div>
      <div className="tf-node__actions">
        <button
          title="Edit"
          aria-label={`Edit ${displayName}`}
          onClick={(e) => { e.stopPropagation(); onEdit(canvasNode.id); }}
          className="tf-icon-btn"
        >
          <Pencil size={13} />
        </button>
        <button
          title="Remove"
          aria-label={`Remove ${displayName}`}
          onClick={(e) => { e.stopPropagation(); onRemove(canvasNode.id); }}
          className="tf-icon-btn tf-icon-btn--danger"
        >
          <Trash2 size={13} />
        </button>
      </div>
      <Handle type="source" position={Position.Bottom} style={{ visibility: "hidden" }} />
    </div>
  );
}
