import { useState } from "react";
import { GitBranch, Loader2, MessageSquareWarning, Pencil } from "lucide-react";
import { StatusBadge } from "./StatusBadge";
import { useChangesetList } from "../hooks/useReview";

function fmt(iso?: string | null) {
  if (!iso) return "";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "" : d.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}

/** Status tracking: every changeset with its outcome (branch or reject reason). */
export function ChangesetsView({
  projectId,
  refreshKey,
  onRevise,
}: {
  projectId: string;
  refreshKey: number;
  /** Opens the changeset as a new draft on the canvas. */
  onRevise: (csId: string, env: string) => Promise<void>;
}) {
  const { rows, loading, error } = useChangesetList(projectId, refreshKey);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [reviseError, setReviseError] = useState<string | null>(null);

  const revise = async (id: string, env: string) => {
    setBusyId(id);
    setReviseError(null);
    try {
      await onRevise(id, env);
    } catch (e) {
      setReviseError(e instanceof Error ? e.message : "Could not open the changeset");
    } finally {
      setBusyId(null);
    }
  };

  return (
    <div className="min-h-0 flex-1 overflow-y-auto bg-[var(--tf-color-canvas)] p-6">
      <div className="mx-auto max-w-4xl">
        <h2 className="mb-1 text-xl font-semibold">Changesets</h2>
        <p className="mb-5 text-sm text-[var(--tf-color-text-muted)]">
          Where each change stands: draft, awaiting review, approved (with its branch) or rejected.
        </p>
        {loading && <Loader2 className="size-5 animate-spin text-[var(--tf-color-text-muted)]" />}
        {(error || reviseError) && <p className="mb-3 text-sm text-[var(--tf-color-error)]">{error ?? reviseError}</p>}
        {!loading && rows.length === 0 && (
          <p className="rounded-xl border border-dashed border-[var(--tf-color-border-strong)] p-10 text-center text-sm text-[var(--tf-color-text-muted)]">
            No changesets yet. Drag modules onto the canvas and publish.
          </p>
        )}
        <ul className="m-0 list-none space-y-3 p-0">
          {rows.map((r) => (
            <li
              key={r.id}
              className="rounded-xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] p-4 shadow-sm"
            >
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="font-medium">{r.title || "Untitled changeset"}</span>
                <div className="flex items-center gap-2">
                  <StatusBadge status={r.status} />
                  {r.status !== "draft" && (
                    <button
                      className="tf-btn tf-btn--ghost"
                      style={{ minHeight: 28, fontSize: 12 }}
                      disabled={busyId === r.id}
                      onClick={() => revise(r.id, r.env)}
                      title="Open as a new draft to add, remove or change resources"
                    >
                      {busyId === r.id ? <Loader2 className="size-3.5 animate-spin" /> : <Pencil className="size-3.5" />}
                      Edit
                    </button>
                  )}
                </div>
              </div>
              <div className="mt-1 text-xs text-[var(--tf-color-text-muted)]">
                {r.env} · updated {fmt(r.updated_at)}
              </div>
              {r.status === "approved" && r.branch_name && (
                <div className="mt-3 flex items-center gap-2 text-sm text-[var(--tf-color-success)]">
                  <GitBranch className="size-4" />
                  <code>{r.branch_name}</code>
                  {r.commit_sha && <span className="font-mono text-xs">@ {r.commit_sha.slice(0, 8)}</span>}
                </div>
              )}
              {r.status === "rejected" && r.reject_reason && (
                <div className="mt-3 flex items-start gap-2 text-sm text-[var(--tf-color-error)]">
                  <MessageSquareWarning className="mt-0.5 size-4 shrink-0" />
                  <span>{r.reject_reason}</span>
                </div>
              )}
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
