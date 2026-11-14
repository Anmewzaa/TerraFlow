import type { NodeProps } from "@xyflow/react";
import { displayName as nodeDisplayName } from "../../lib/valueText";
import { Pencil, Trash2 } from "lucide-react";
import type { CanvasNode } from "../../types/changeset";
import type { Issue } from "../../lib/validate";
import { colorValue } from "../../lib/canvasLayout";
import { getAzureIcon } from "./domainStyles";

interface DraftContainerData {
  canvasNode: CanvasNode;
  title: string;
  onEdit: (id: string) => void;
  onRemove: (id: string) => void;
  childCount: number;
  issues?: Issue[];
  /** The project's policy warnings for this resource (8.13). */
  warnings?: Issue[];
  [key: string]: unknown;
}

/** A draft resource that can hold other resources (resource group, VNet, subnet). */
export function DraftContainerNode({ data, selected }: NodeProps) {
  const { canvasNode, title, onEdit, onRemove, childCount, issues = [], warnings = [] } = data as DraftContainerData;
  const invalid = issues.length > 0;
  const color = colorValue(canvasNode.color);
  const displayName = nodeDisplayName(canvasNode.values, canvasNode.instance_name);
  const icon = getAzureIcon(canvasNode.module_name);

  return (
    <div
      className={`tf-block${selected ? " tf-block--selected" : ""}${invalid ? " tf-block--invalid" : ""}`}
      title={invalid ? issues.map((i) => i.text).join("\n") : undefined}
      style={{ ["--blk" as string]: color }}
    >
      <div className="tf-block__head">
        <span className="tf-block__chip">
          {icon ? <img src={icon} alt="" /> : <span className="tf-block__dot" />}
        </span>
        <div className="tf-block__titles">
          <span className="tf-block__title">{title}</span>
          <span className="tf-block__name">{displayName}</span>
        </div>
        {invalid ? <span className="tf-invalid-badge">{issues.length} to fix</span> : warnings.length > 0 ? <span className="tf-warn-badge">{warnings.length} to review</span> : <span className="tf-draft-badge">draft</span>}
        <div className="tf-block__actions">
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
      </div>
      {childCount === 0 && <div className="tf-block__empty">Drop resources here</div>}
    </div>
  );
}
