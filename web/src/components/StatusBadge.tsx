const TONE: Record<string, { tone: string; label: string }> = {
  draft: { tone: "neutral", label: "Draft" },
  published: { tone: "info", label: "Awaiting review" },
  approved: { tone: "good", label: "Approved" },
  rejected: { tone: "bad", label: "Rejected" },
};

export function StatusBadge({ status }: { status: string }) {
  const t = TONE[status] ?? { tone: "neutral", label: status };
  return (
    <span className="tf-badge" data-tone={t.tone}>
      {t.label}
    </span>
  );
}
