import { Handle, Position, type NodeProps } from "@xyflow/react";
import type { CanvasNode } from "../../types/changeset";
import { domainCssVars, domainBgOpacity, getAzureIcon, moduleDomainMap } from "./domainStyles";

interface DraftNodeData {
  canvasNode: CanvasNode;
  onEdit: (id: string) => void;
  onRemove: (id: string) => void;
  [key: string]: unknown;
}

export function DraftNode({ data }: NodeProps) {
  const { canvasNode } = data as DraftNodeData;
  const domain = moduleDomainMap[canvasNode.module_name] ?? "neutral";
  const color = domainCssVars[domain] ?? domainCssVars.neutral;
  const iconBg = domainBgOpacity[domain] ?? domainBgOpacity.neutral;
  const iconSrc = getAzureIcon(canvasNode.module_name);
  const displayName = (canvasNode.values?.name as string) || canvasNode.instance_name;
  const displayTitle = formatTitle(canvasNode.module_name);

  return (
    <div className="tf-node tf-node--draft">
      <Handle type="target" position={Position.Top} style={{ visibility: "hidden" }} />
      <div className="tf-node__accent" style={{ background: "var(--tf-color-accent)" }} />
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
        <div>
          <div className="tf-node__title">{displayTitle}</div>
          <div className="tf-node__type">
            <span className="tf-node__draft-badge">draft</span>
          </div>
        </div>
      </div>
      <div className="tf-node__body">
        <span className="tf-node__ident">{displayName}</span>
        <div className="tf-node__meta">
          <span>{canvasNode.module_version}</span>
          <span>·</span>
          <span>{canvasNode.target_folder}/</span>
        </div>
      </div>
      <Handle type="source" position={Position.Bottom} style={{ visibility: "hidden" }} />
    </div>
  );
}

function formatTitle(name: string): string {
  return name
    .replace(/[-_]/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase());
}
