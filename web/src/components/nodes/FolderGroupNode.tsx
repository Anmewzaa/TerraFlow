import type { NodeProps } from "@xyflow/react";

export function FolderGroupNode({ data }: NodeProps) {
  const label = data.label as string;
  return (
    <div
      style={{
        width: "100%",
        height: "100%",
        background: "var(--tf-color-canvas)",
        border: "1px dashed var(--tf-color-border)",
        borderRadius: "var(--tf-radius-l)",
        position: "relative",
      }}
    >
      <div
        style={{
          position: "absolute",
          top: 8,
          left: 12,
          fontSize: 11,
          fontWeight: 600,
          color: "var(--tf-color-text-secondary)",
          textTransform: "uppercase",
          letterSpacing: "0.05em",
        }}
      >
        {label}
      </div>
    </div>
  );
}
