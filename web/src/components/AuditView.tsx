import { useEffect, useMemo, useState } from "react";
import { ArrowLeft, ArrowRight, Loader2, Search } from "lucide-react";
import { useAudit } from "../hooks/useReview";

const ACTION: Record<string, { label: string; tone: string }> = {
  "changeset.publish": { label: "Published", tone: "info" },
  "changeset.approve": { label: "Approved", tone: "good" },
  "changeset.reject": { label: "Rejected", tone: "bad" },
};

function summary(detail: Record<string, unknown> | null, resource?: string): string {
  const parts: string[] = [];
  if (resource) parts.push(resource);
  if (!detail) return parts.join(" · ");
  if (detail.title) parts.push(String(detail.title));
  if (detail.branch) parts.push(`branch ${detail.branch}`);
  if (detail.reason) parts.push(`reason: ${detail.reason}`);
  return parts.join(" · ");
}

const PAGE_SIZE = 10;
const field = "h-9 rounded-lg border border-[var(--tf-color-border-strong)] bg-[var(--tf-color-surface)] px-2.5 text-sm outline-none focus:border-[var(--tf-color-action)]";

/** Who did what, when. Newest first, ten to a page, with a date range, an action and a text filter. */
export function AuditView({ projectId = null, refreshKey = 0 }: { projectId?: string | null; refreshKey?: number }) {
  const { entries, loading, error } = useAudit(projectId, refreshKey);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [action, setAction] = useState("");
  const [q, setQ] = useState("");
  const [page, setPage] = useState(0);

  const actions = useMemo(() => [...new Set(entries.map((e) => e.action))].sort(), [entries]);
  const filtered = useMemo(() => {
    const start = from ? new Date(`${from}T00:00:00`).getTime() : -Infinity;
    const end = to ? new Date(`${to}T23:59:59.999`).getTime() : Infinity;
    const text = q.trim().toLowerCase();
    return entries.filter((e) => {
      const t = new Date(e.created_at).getTime();
      if (t < start || t > end) return false;
      if (action && e.action !== action) return false;
      return !text || `${e.user_oid} ${e.action} ${summary(e.detail, e.resource)}`.toLowerCase().includes(text);
    });
  }, [entries, from, to, action, q]);
  const pages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  useEffect(() => setPage(0), [from, to, action, q]);
  const current = Math.min(page, pages - 1);
  const shown = filtered.slice(current * PAGE_SIZE, current * PAGE_SIZE + PAGE_SIZE);
  const filtering = !!(from || to || action || q);

  return (
    <div className="min-h-0 flex-1 overflow-y-auto bg-[var(--tf-color-canvas)] p-6">
      <div className="mx-auto max-w-5xl">
        <h2 className="mb-1 text-xl font-semibold">Audit log</h2>
        <p className="mb-5 text-sm text-[var(--tf-color-text-muted)]">
          {projectId ? "Every publish, approval and rejection." : "Everything done in TerraFlow: publishing, approvals, projects, modules and raw resources."}
        </p>
        {loading && <Loader2 className="size-5 animate-spin text-[var(--tf-color-text-muted)]" />}
        {error && <p className="text-sm text-[var(--tf-color-error)]">{error}</p>}
        {!loading && !error && entries.length === 0 && (
          <p className="rounded-xl border border-dashed border-[var(--tf-color-border-strong)] p-10 text-center text-sm text-[var(--tf-color-text-muted)]">
            No activity recorded yet.
          </p>
        )}
        {entries.length > 0 && (
          <div className="mb-4 flex flex-wrap items-end gap-3">
            <label className="grid gap-1 text-xs text-[var(--tf-color-text-muted)]">
              From
              <input type="date" value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} className={field} />
            </label>
            <label className="grid gap-1 text-xs text-[var(--tf-color-text-muted)]">
              To
              <input type="date" value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} className={field} />
            </label>
            <label className="grid gap-1 text-xs text-[var(--tf-color-text-muted)]">
              Action
              <select value={action} onChange={(e) => setAction(e.target.value)} className={field}>
                <option value="">All actions</option>
                {actions.map((a) => (
                  <option key={a} value={a}>{ACTION[a]?.label ?? a}</option>
                ))}
              </select>
            </label>
            <label className="relative grid min-w-[12rem] flex-1 gap-1 text-xs text-[var(--tf-color-text-muted)]">
              Search
              <Search className="pointer-events-none absolute bottom-2.5 left-2.5 size-4" aria-hidden />
              <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Who, resource or detail" className={`${field} pl-8`} />
            </label>
            {filtering && (
              <button type="button" onClick={() => { setFrom(""); setTo(""); setAction(""); setQ(""); }} className="h-9 rounded-lg px-3 text-sm text-[var(--tf-color-action)] hover:bg-[var(--tf-glass-strong)]">
                Clear filters
              </button>
            )}
          </div>
        )}
        {entries.length > 0 && filtered.length === 0 && (
          <p className="rounded-xl border border-dashed border-[var(--tf-color-border-strong)] p-10 text-center text-sm text-[var(--tf-color-text-muted)]">
            Nothing matches these filters.
          </p>
        )}
        {filtered.length > 0 && (
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
                {shown.map((e) => {
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
                      <td className="px-4 py-2.5 text-[var(--tf-color-text-secondary)]">{summary(e.detail, e.resource)}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
            <div className="flex items-center justify-between gap-3 border-t border-[var(--tf-color-border)] px-4 py-3 text-sm text-[var(--tf-color-text-muted)]">
              <span>
                {current * PAGE_SIZE + 1}-{current * PAGE_SIZE + shown.length} of {filtered.length}
                {filtering && ` (${entries.length} in all)`}
              </span>
              <div className="flex items-center gap-2">
                <button type="button" onClick={() => setPage(current - 1)} disabled={current === 0} aria-label="Newer entries" className="grid size-8 place-items-center rounded-full border border-[var(--tf-color-border-strong)] hover:border-[var(--tf-color-action)] disabled:cursor-not-allowed disabled:opacity-40">
                  <ArrowLeft className="size-4" />
                </button>
                <span>Page {current + 1} of {pages}</span>
                <button type="button" onClick={() => setPage(current + 1)} disabled={current >= pages - 1} aria-label="Older entries" className="grid size-8 place-items-center rounded-full border border-[var(--tf-color-border-strong)] hover:border-[var(--tf-color-action)] disabled:cursor-not-allowed disabled:opacity-40">
                  <ArrowRight className="size-4" />
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
