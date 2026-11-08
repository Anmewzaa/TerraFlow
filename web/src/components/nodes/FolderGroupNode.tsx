import type { NodeProps } from "@xyflow/react";
import type { GraphNode } from "../../types/graph";

export function FolderGroupNode({ data }: NodeProps) {
  const label = data.label as string;
  const node = data.node as GraphNode | undefined;
  const folder = node?.folder;

  return (
    <div className="tf-container-node">
      <div className="tf-container-node__label">
        <span className="tf-dot" style={{ background: "var(--tf-cat-neutral)" }} />
        {label}
        {folder && <code>{folder}/</code>}
      </div>
    </div>
  );
}
