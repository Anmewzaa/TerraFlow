import { Loader2 } from "lucide-react";
import { useAudit } from "../hooks/useReview";

const ACTION: Record<string, { label: string; tone: string }> = {
  "changeset.publish": { label: "Published", tone: "info" },
  "changeset.approve": { label: "Approved", tone: "good" },
  "changeset.reject": { label: "Rejected", tone: "bad" },
};

function summary(detail: Record<string, unknown> | null): string {
  if (!detail) return "";
  const parts: string[] = [];
  if (detail.title) parts.push(String(detail.title));
  if (detail.branch) parts.push(`branch ${detail.branch}`);
  if (detail.reason) parts.push(`reason: ${detail.reason}`);
  return parts.join(" · ");
}

/** Who did what, when. Newest first. */
export function AuditView({ projectId, refreshKey }: { projectId: string; refreshKey: number }) {
  const { entries, loading, error } = useAudit(projectId, refreshKey);

  return (
    <div className="min-h-0 flex-1 overflow-y-auto bg-[var(--tf-color-canvas)] p-6">
      <div className="mx-auto max-w-5xl">
        <h2 className="mb-1 text-xl font-semibold">Audit log</h2>
        <p className="mb-5 text-sm text-[var(--tf-color-text-muted)]">Every publish, approval and rejection.</p>
        {loading && <Loader2 className="size-5 animate-spin text-[var(--tf-color-text-muted)]" />}
        {error && <p className="text-sm text-[var(--tf-color-error)]">{error}</p>}
        {!loading && !error && entries.length === 0 && (
          <p className="rounded-xl border border-dashed border-[var(--tf-color-border-strong)] p-10 text-center text-sm text-[var(--tf-color-text-muted)]">
            No activity recorded yet.
          </p>
        )}
        {entries.length > 0 && (
          <div className="overflow-hidden rounded-xl border border-[var(--tf-color-border)] bg-[var(--tf-color-surface)] shadow-sm">
            <table className="w-full border-collapse text-left text-sm">
              <thead className="bg-[var(--tf-color-surface-subtle)] text-xs uppercase tracking-wide text-[var(--tf-color-text-muted)]">
                <tr>
                  <th className="px-4 py-2.5 font-semibold">When</th>
                  <th className="px-4 py-2.5 font-semibold">Who</th>
                  <th className="px-4 py-2.5 font-semibold">Action</th>
                  <th className="px-4 py-2.5 font-semibold">Details</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((e) => {
                  const a = ACTION[e.action] ?? { label: e.action, tone: "neutral" };
                  return (
                    <tr key={e.id} className="border-t border-[var(--tf-color-border)] align-top">
                      <td className="whitespace-nowrap px-4 py-2.5 text-[var(--tf-color-text-secondary)]">
                        {new Date(e.created_at).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" })}
                      </td>
                      <td className="px-4 py-2.5 font-mono text-xs">{e.user_oid}</td>
                      <td className="px-4 py-2.5">
                        <span className="tf-badge" data-tone={a.tone}>
                          {a.label}
                        </span>
                      </td>
                      <td className="px-4 py-2.5 text-[var(--tf-color-text-secondary)]">{summary(e.detail)}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
