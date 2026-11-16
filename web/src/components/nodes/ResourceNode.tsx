import { Handle, Position, type NodeProps } from "@xyflow/react";
import type { GraphNode } from "../../types/graph";
import { ResourceGlyph } from "./ResourceGlyph";
import { domainCssVars, domainBgOpacity, getAzureIcon } from "./domainStyles";

export function ResourceNode({ data }: NodeProps) {
  const node = data.node as GraphNode;
  const domain = node.domain ?? "neutral";
  const color = domainCssVars[domain] ?? domainCssVars.neutral;
  const iconBg = domainBgOpacity[domain] ?? domainBgOpacity.neutral;
  const iconSrc = getAzureIcon(node.type);
  const version = node.source ? extractVersion(node.source) : null;
  const displayTitle = formatTitle(node.type);

  return (
    <div className="tf-node">
      <Handle type="target" position={Position.Top} style={{ visibility: "hidden" }} />
      <div className="tf-node__accent" style={{ background: color }} />
      <div className="tf-node__header">
        <div className="tf-node__icon" style={{ background: iconBg, color }}>
          {iconSrc ? (
            <img src={iconSrc} alt="" />
          ) : (
            <ResourceGlyph domain={domain} className="size-4" />
          )}
        </div>
        <div>
          <div className="tf-node__title">{displayTitle}</div>
          <div className="tf-node__type">{node.kind}</div>
        </div>
      </div>
      <div className="tf-node__body">
        {node.nameState === "resolved" ? (
          <span className="tf-node__ident">{node.name}</span>
        ) : (
          <span className="tf-unknown" style={{ fontSize: 11 }}>unknown</span>
        )}
        {version && (
          <div className="tf-node__meta">
            <span>{version}</span>
          </div>
        )}
      </div>
      <Handle type="source" position={Position.Bottom} style={{ visibility: "hidden" }} />
    </div>
  );
}

function extractVersion(source: string): string | null {
  const m = source.match(/\?ref=([^&]+)/);
  return m ? m[1] : null;
}

function formatTitle(type: string): string {
  return type
    .replace(/[-_]/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase());
}
