interface Props {
  draftCount: number;
  onReviewDiff: () => void;
  onSave: () => void;
}

export function ChangesetBar({ draftCount, onReviewDiff, onSave }: Props) {
  if (draftCount === 0) return null;

  return (
    <div className="tf-changeset-bar">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
        <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
        <polyline points="14 2 14 8 20 8" />
      </svg>
      <span className="tf-changeset-bar__count">
        <strong>{draftCount}</strong> draft change{draftCount !== 1 ? "s" : ""}
      </span>
      <button
        className="tf-btn tf-btn--ghost"
        style={{ height: 28, padding: "0 var(--tf-space-2)", fontSize: 12 }}
        onClick={onReviewDiff}
      >
        Review diff
      </button>
      <button
        className="tf-btn"
        style={{ height: 28, padding: "0 var(--tf-space-3)", fontSize: 12, background: "var(--tf-color-success)" }}
        onClick={onSave}
      >
        Save version
      </button>
    </div>
  );
}
