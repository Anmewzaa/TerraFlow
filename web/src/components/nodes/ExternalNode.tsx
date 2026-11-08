import { Handle, Position, type NodeProps } from "@xyflow/react";
import type { GraphExternal } from "../../types/graph";

export function ExternalNode({ data }: NodeProps) {
  const ext = data.external as GraphExternal;

  return (
    <div className="tf-node" style={{ opacity: 0.85, borderStyle: "dashed", borderColor: "var(--tf-color-text-muted)" }}>
      <Handle type="target" position={Position.Top} style={{ visibility: "hidden" }} />
      <div className="tf-node__accent" style={{ background: "var(--tf-cat-neutral)" }} />
      <div className="tf-node__header">
        <div className="tf-node__icon" style={{ background: "rgba(107,117,133,0.08)", color: "var(--tf-cat-neutral)" }}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
            <polyline points="15 3 21 3 21 9" />
            <line x1="10" y1="14" x2="21" y2="3" />
          </svg>
        </div>
        <div>
          <div className="tf-node__title">{ext.kind}</div>
          <div className="tf-node__type">external</div>
        </div>
      </div>
      <div className="tf-node__body">
        <span className="tf-node__ident">{ext.name}</span>
        <div className="tf-node__meta">
          <span>source: {ext.source}</span>
        </div>
      </div>
      <Handle type="source" position={Position.Bottom} style={{ visibility: "hidden" }} />
    </div>
  );
}
