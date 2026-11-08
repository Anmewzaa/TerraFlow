import { useState } from "react";
import { Loader2, Send, X } from "lucide-react";

interface Props {
  draftCount: number;
  env: string;
  /** Pre-fills the title, e.g. the name of the saved version being published. */
  initialTitle?: string;
  /** Resolves true when the changeset was published, else the reason. */
  onPublish: (title: string) => Promise<true | string>;
  onClose: () => void;
  /** Another changeset of this project that still waits for review: it must be decided first. */
  blockedBy?: { id: string; title: string } | null;
  onOpenReview?: () => void;
}

/** Replaces window.prompt, which is blocked in embedded browsers. */
export function PublishDialog({ draftCount, env, initialTitle = "", onPublish, onClose, blockedBy, onOpenReview }: Props) {
  const [title, setTitle] = useState(initialTitle);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const r = await onPublish(title.trim());
      if (r === true) onClose();
      else setError(r);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not publish");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-[var(--tf-color-overlay)] p-4 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
      aria-label="Publish changeset"
    >
      <form
        onSubmit={submit}
        className="w-full max-w-md rounded-2xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] p-6 shadow-2xl"
      >
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold">Send for review</h2>
          <button type="button" onClick={onClose} aria-label="Close" className="text-[var(--tf-color-text-muted)]">
            <X className="size-5" />
          </button>
        </div>
        <p className="mb-4 text-sm text-[var(--tf-color-text-secondary)]">
          {draftCount} change{draftCount !== 1 ? "s" : ""} in <strong>{env}</strong> go to the Review queue. An approver
          decides; nothing is written to the repository until then.
        </p>
        {blockedBy && (
          <div role="alert" className="mb-4 rounded-lg border border-[var(--tf-color-warning,var(--tf-color-border-strong))] bg-[var(--tf-color-surface-raised)] px-3 py-2.5 text-sm">
            <strong>Decide the earlier changeset first.</strong>{" "}
            “{blockedBy.title || "Untitled changeset"}” is still waiting for review. Approve it (so it becomes a branch) or
            reject it, then publish this one.
            {onOpenReview && (
              <div className="mt-2">
                <button type="button" className="tf-btn tf-btn--ghost" onClick={() => { onClose(); onOpenReview(); }}>
                  Open Review
                </button>
              </div>
            )}
          </div>
        )}
        <label htmlFor="cs-title" className="mb-1 block text-sm text-[var(--tf-color-text-secondary)]">
          Changeset title
        </label>
        <input
          id="cs-title"
          autoFocus
          required
          maxLength={120}
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="e.g. Add storage account for reports"
          className="w-full rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-canvas)] px-3 py-2 text-sm outline-none focus:border-[var(--tf-color-action)]"
        />
        {error && (
          <p role="alert" className="mt-3 text-sm text-[var(--tf-color-error)]">
            {error}
          </p>
        )}
        <div className="mt-5 flex justify-end gap-2">
          <button type="button" onClick={onClose} className="tf-btn tf-btn--ghost">
            Cancel
          </button>
          <button type="submit" disabled={busy || !title.trim() || !!blockedBy} className="tf-btn">
            {busy ? <Loader2 className="size-4 animate-spin" /> : <Send className="size-4" />} Publish
          </button>
        </div>
      </form>
    </div>
  );
}
