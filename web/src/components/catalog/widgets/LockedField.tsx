interface Props {
  value: unknown;
  reason?: string;
}

export function LockedField({ value, reason }: Props) {
  return (
    <div
      style={{
        padding: "var(--tf-space-2)",
        background: "var(--tf-color-surface-raised)",
        border: "1px solid var(--tf-color-border)",
        borderRadius: "var(--tf-radius-s)",
        fontSize: "var(--tf-text-small)",
        color: "var(--tf-color-text-muted)",
        display: "flex",
        alignItems: "center",
        gap: "var(--tf-space-2)",
      }}
    >
      <span>{String(value ?? "")}</span>
      {reason && (
        <span
          style={{
            fontSize: "var(--tf-text-xs)",
            color: "var(--tf-color-warning)",
          }}
          title={reason}
        >
          (locked)
        </span>
      )}
    </div>
  );
}
